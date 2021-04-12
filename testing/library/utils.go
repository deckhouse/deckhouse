package library

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/iancoleman/strcase"
	"github.com/imdario/mergo"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/testing/library/git"
)

func GetModulesImagesTags(modulePath string) (map[string]map[string]string, error) {
	var (
		tags      map[string]map[string]string
		searchGit bool
	)

	fi, err := os.Stat(filepath.Join(filepath.Dir(modulePath), "images_tags.json"))
	if err != nil || fi.Size() == 0 {
		searchGit = true
	}

	if searchGit {
		tags, err = getModulesImagesTagsFromGit(modulePath)
	} else {
		tags, err = getModulesImagesTagsFromLocalPath(modulePath)
	}
	if err != nil {
		return nil, err
	}

	return tags, err
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

func getModulesImagesTagsFromGit(modulePath string) (map[string]map[string]string, error) {
	tags := make(map[string]map[string]string)

	for _, path := range []string{modulePath, filepath.Join(filepath.Dir(modulePath), "000-common")} {
		imageTags, err := git.ListTreeObjects(filepath.Join(path, "images"))
		if err != nil {
			return nil, err
		}

		_, moduleDir := filepath.Split(path)
		moduleDirClean := string([]byte(moduleDir)[4:])
		moduleName := strcase.ToLowerCamel(moduleDirClean)
		tags[moduleName] = make(map[string]string)

		for _, tag := range imageTags {
			fileName := strcase.ToLowerCamel(tag.File)
			tags[moduleName][fileName] = tag.Object
		}
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
	globalValuesRaw, err := ioutil.ReadFile(filepath.Join("/deckhouse", "modules", "values-default.yaml"))
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
