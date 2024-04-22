/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package conversion

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/itchyny/gojq"
	"sigs.k8s.io/yaml"
)

type Converter struct {
	mtx sync.Mutex

	latest      int
	conversions map[int]string
}

func newConverter(pathToConversions string) (*Converter, error) {
	c := &Converter{conversions: make(map[int]string), latest: 1}
	conversionsDir, err := os.ReadDir(pathToConversions)
	if err != nil {
		return nil, err
	}
	for _, file := range conversionsDir {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yaml" {
			continue
		}
		v, conversion, err := readConversions(path.Join(pathToConversions, file.Name()))
		if err != nil {
			return nil, err
		}
		if v > c.latest {
			c.latest = v
		}
		c.conversions[v] = conversion
	}
	return c, err
}
func readConversions(path string) (int, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	var parsed struct {
		Version     int
		Conversions []string
	}
	if err = yaml.Unmarshal(data, &parsed); err != nil {
		return 0, "", err
	}
	return parsed.Version, strings.Join(parsed.Conversions, " | "), nil
}

func (c *Converter) LatestVersion() int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.latest
}

func (c *Converter) IsKnownVersion(version int) bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.conversions != nil {
		if _, has := c.conversions[version]; has {
			return true
		}
	}
	return version == c.latest || version == 1
}

func (c *Converter) ListVersionsWithoutLatest() []int {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	versions := make([]int, 0)
	if c.conversions == nil {
		return versions
	}
	for ver := range c.conversions {
		if ver != c.latest {
			versions = append(versions, ver)
		}
	}
	return versions
}

func (c *Converter) ConvertToLatest(currentVersion int, settings map[string]interface{}) (int, map[string]interface{}, error) {
	return c.ConvertTo(currentVersion, c.latest, settings)
}

func (c *Converter) ConvertTo(currentVersion, version int, settings map[string]interface{}) (int, map[string]interface{}, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if currentVersion == c.latest || settings == nil || c.conversions == nil {
		return currentVersion, settings, nil
	}
	if version == 0 {
		version = c.latest
	}
	var err error
	for currentVersion++; currentVersion <= version; currentVersion++ {
		if settings, err = c.convert(currentVersion, settings); err != nil {
			return currentVersion, nil, err
		}
	}
	return c.latest, settings, err
}
func (c *Converter) convert(version int, settings map[string]interface{}) (map[string]interface{}, error) {
	conversion := c.conversions[version]
	if conversion == "" {
		return nil, errors.New("conversion not found")
	}
	query, err := gojq.Parse(conversion)
	if err != nil {
		return nil, err
	}
	v, _ := query.Run(settings).Next()
	if err, ok := v.(error); ok {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	filtered, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.New("cannot unmarshal after converting")
	}
	return filtered, nil
}

func TestConvert(rawSettings, rawExpected, pathToConversions string, currentVersion, version int) error {
	converter, err := newConverter(pathToConversions)
	if err != nil {
		return err
	}

	settings, err := readSettings(rawSettings)
	if err != nil {
		return err
	}
	_, converted, err := converter.ConvertTo(currentVersion, version, settings)
	if err != nil {
		return err
	}
	marshaledConverted, err := json.Marshal(converted)
	if err != nil {
		return err
	}

	expected, err := readSettings(rawExpected)
	if err != nil {
		return err
	}
	marshaledExpected, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if string(marshaledConverted) != string(marshaledExpected) {
		return fmt.Errorf("expected: %s got: %s\n", marshaledExpected, marshaledConverted)
	}
	return nil
}
func readSettings(settings string) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	err := yaml.Unmarshal([]byte(settings), &parsed)
	return parsed, err
}
