package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/tidwall/gjson"

	"github.com/deckhouse/deckhouse/testing/library/git"

	"github.com/segmentio/go-camelcase"

	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"
)

func InitValues(modulePath string, userDefinedValuesRaw []byte) (map[string]interface{}, error) {
	var (
		err error

		testsValues        map[string]interface{}
		moduleValues       map[string]interface{}
		moduleImagesValues map[string]map[string]map[string]map[string]map[string]string
		userDefinedValues  map[string]interface{}
		finalValues        = new(map[string]interface{})
	)

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
	imageTagsRaw, err := ioutil.ReadFile(filepath.Join(filepath.Dir(modulePath), "images_tags.json"))
	if err != nil {
		moduleImagesValues = map[string]map[string]map[string]map[string]map[string]string{
			"global": {
				"modulesImages": {
					"tags": {},
				},
			},
		}

		imageTags, err := git.ListTreeObjects(filepath.Join(modulePath, "images"))
		if err != nil {
			return nil, err
		}

		_, moduleDir := filepath.Split(modulePath)
		moduleDirClean := string([]byte(moduleDir)[4:])
		moduleName := camelcase.Camelcase(moduleDirClean)
		moduleImagesValues["global"]["modulesImages"]["tags"][moduleName] = map[string]string{}
		for _, tag := range imageTags {
			moduleImagesValues["global"]["modulesImages"]["tags"][moduleName][tag.File] = tag.Object
		}
	} else {
		var imageTags map[string]map[string]string
		err = json.Unmarshal(imageTagsRaw, &imageTags)
		if err != nil {
			return nil, fmt.Errorf("can't unmarshal JSON: %s\n%s", err, imageTagsRaw)
		}

		moduleImagesValues = map[string]map[string]map[string]map[string]map[string]string{
			"global": {
				"modulesImages": {
					"tags": imageTags,
				},
			},
		}
	}

	// 4. Get user-supplied values
	err = yaml.Unmarshal(userDefinedValuesRaw, &userDefinedValues)
	if err != nil {
		return nil, err
	}

	err = mergeValues(finalValues, moduleValues, testsValues, moduleImagesValues, userDefinedValues)
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
	var result []string
	for _, element := range kr.Array() {
		result = append(result, element.String())
	}

	return result
}
