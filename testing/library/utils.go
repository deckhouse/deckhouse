/*
Copyright 2021 Flant JSC

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

package library

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/imdario/mergo"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
)

var tags map[string]map[string]string

func init() {
	tags = make(map[string]map[string]string)
	for _, pattern := range []string{"/deckhouse/modules/*", "/deckhouse/ee/modules/*", "/deckhouse/ee/fe/modules/*"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			panic(err)
		}

		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil {
				panic(err)
			}
			if !info.IsDir() {
				continue
			}

			parts := strings.SplitN(info.Name(), "-", 2)
			tags[strcase.ToLowerCamel(parts[1])] = make(map[string]string)
		}
	}
}

func GetModulesImagesTags(modulePath string) (map[string]map[string]string, error) {
	var (
		modulesTags map[string]map[string]string
		search      bool
	)

	if fi, err := os.Stat(filepath.Join(filepath.Dir(modulePath), "images_tags.json")); err != nil || fi.Size() == 0 {
		search = true
	}

	if search {
		modulesTags = tags
	} else {
		var err error

		modulesTags, err = getModulesImagesTagsFromLocalPath(modulePath)
		if err != nil {
			return nil, err
		}
	}

	return modulesTags, nil
}

func getModulesImagesTagsFromLocalPath(modulePath string) (map[string]map[string]string, error) {
	var tags map[string]map[string]string

	imageTagsRaw, err := ioutil.ReadFile(filepath.Join(filepath.Dir(modulePath), "images_tags.json"))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(imageTagsRaw, &tags)
	if err != nil {
		return nil, err
	}

	return tags, nil
}

func InitValues(modulePath string, userDefinedValuesRaw []byte) (map[string]interface{}, error) {
	var (
		err error

		testsValues        map[string]interface{}
		moduleValues       map[string]interface{}
		globalValues       map[string]interface{}
		moduleImagesValues map[string]map[string]map[string]map[string]map[string]string
		userDefinedValues  map[string]interface{}
		finalValues        = new(map[string]interface{})
	)

	// 0. Get values from values-default.yaml
	globalValuesRaw, err := ioutil.ReadFile(filepath.Join("/deckhouse", "modules", "values.yaml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = yaml.Unmarshal(globalValuesRaw, &globalValues)
	if err != nil {
		return nil, err
	}

	// 1. Get values from modules/[module_name]/template_tests/values.yaml
	testsValuesRaw, err := ioutil.ReadFile(filepath.Join(modulePath, "template_tests", "values.yaml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = yaml.Unmarshal(testsValuesRaw, &testsValues)
	if err != nil {
		return nil, err
	}

	// 2. Get values from modules/[module_name]/values.yaml
	moduleValuesRaw, err := ioutil.ReadFile(filepath.Join(modulePath, "values.yaml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = yaml.Unmarshal(moduleValuesRaw, &moduleValues)
	if err != nil {
		return nil, err
	}

	// 3. Get image tags
	tags, err := GetModulesImagesTags(modulePath)
	if err != nil {
		return nil, err
	}
	moduleImagesValues = map[string]map[string]map[string]map[string]map[string]string{
		"global": {
			"modulesImages": {
				"tags": tags,
			},
		},
	}

	// 4. Get user-supplied values
	err = yaml.Unmarshal(userDefinedValuesRaw, &userDefinedValues)
	if err != nil {
		return nil, err
	}

	err = mergeValues(finalValues, moduleValues, testsValues, globalValues, moduleImagesValues, userDefinedValues)
	if err != nil {
		return nil, err
	}

	return *finalValues, nil
}

func mergeValues(final *map[string]interface{}, iterations ...interface{}) error {
	for _, valuesStructure := range iterations {
		if valuesStructure != nil {
			var newMap = map[string]interface{}{}

			v := reflect.ValueOf(valuesStructure)
			if v.Kind() == reflect.Map {
				for _, key := range v.MapKeys() {
					val := v.MapIndex(key)
					newMap[key.String()] = val.Interface()
				}
			}
			err := mergo.Merge(final, newMap, mergo.WithOverride)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// refactor into a "store" package

type KubeResult struct {
	gjson.Result
}

func (kr KubeResult) AsStringSlice() []string {
	array := kr.Array()

	result := make([]string, 0, len(array))
	for _, element := range array {
		result = append(result, element.String())
	}

	return result
}

func (kr KubeResult) DropFields(fields ...string) KubeResult {
	if !kr.IsObject() {
		return kr
	}
	// Ignored fields index:
	// - array with zero length -> fully ignored path
	// - array -> field has subpaths to ignore
	fieldsIdx := map[string][]string{}
	for _, v := range fields {
		parts := strings.SplitN(v, ".", 2)
		root := parts[0]
		// Field is fully ignored, its subpathes are not important now.
		if len(parts) == 1 {
			fieldsIdx[root] = make([]string, 0)
		}

		if v, ok := fieldsIdx[root]; ok {
			// Index has zero length array, do not append subpaths.
			if len(v) == 0 {
				continue
			}
		} else {
			fieldsIdx[root] = make([]string, 0)
		}
		if len(parts) > 1 {
			fieldsIdx[root] = append(fieldsIdx[root], parts[1])
		}
	}

	resMap := map[string]interface{}{}
	kr.ForEach(func(key, value gjson.Result) bool {
		keyStr := key.String()
		newFields, ok := fieldsIdx[keyStr]
		// Non-ignored field
		if !ok {
			resMap[keyStr] = json.RawMessage(value.Raw)
			return true
		}
		// Fully ignored field.
		if len(newFields) == 0 {
			return true
		}
		// Recurse drop for field with ignored subpaths.
		resMap[keyStr] = json.RawMessage(KubeResult{Result: value}.DropFields(newFields...).Raw)
		return true
	})
	mapBytes, _ := json.Marshal(resMap)

	return KubeResult{Result: gjson.ParseBytes(mapBytes)}
}
