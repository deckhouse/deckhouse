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
	c := &Converter{conversions: make(map[int]string)}
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

func (c *Converter) Conversion(version int) string {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.conversions == nil {
		return ""
	}
	return c.conversions[version]
}

func (c *Converter) IsKnownVersion(version int) bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if c.conversions != nil {
		if _, has := c.conversions[version]; has {
			return true
		}
	}
	return version == c.latest
}

func (c *Converter) VersionList() []int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	versions := make([]int, 0)
	if c.conversions == nil {
		return versions
	}
	for ver := range c.conversions {
		versions = append(versions, ver)
	}
	versions = append(versions, c.latest)
	return versions
}
func (c *Converter) PreviousVersionsList() []int {
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
	iter := query.Run(settings)
	var filtered map[string]interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok = v.(error); ok {
			var haltErr *gojq.HaltError
			if errors.As(err, &haltErr) && haltErr.Value() == nil {
				break
			}
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		if filtered, ok = v.(map[string]interface{}); !ok {
			return nil, errors.New("cannot unmarshal after converting")
		}
	}
	return filtered, err
}

func TestConvert(pathToSettings, pathToExpectedSettings, pathToConversions string, currentVersion,
	version int) (map[string]interface{}, map[string]interface{}, bool, error) {
	converter, err := newConverter(pathToConversions)
	if err != nil {
		return nil, nil, false, err
	}

	settings, err := readSettings(pathToSettings)
	if err != nil {
		return nil, nil, false, err
	}
	_, converted, err := converter.ConvertTo(currentVersion, version, settings)
	if err != nil {
		return nil, nil, false, err
	}
	marshaledConverted, err := json.Marshal(converted)
	if err != nil {
		return nil, nil, false, err
	}

	expected, err := readSettings(pathToExpectedSettings)
	if err != nil {
		return nil, nil, false, err
	}
	marshaledExpected, err := json.Marshal(expected)
	if err != nil {
		return nil, nil, false, err
	}
	return converted, expected, string(marshaledConverted) == string(marshaledExpected), nil
}
func readSettings(pathToSettings string) (map[string]interface{}, error) {
	raw, err := os.ReadFile(pathToSettings)
	if err != nil {
		return nil, err
	}
	var parsed map[string]interface{}
	err = yaml.Unmarshal(raw, &parsed)
	return parsed, err
}
