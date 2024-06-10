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

package openapi_validation

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/testing/openapi_validation/validators"
)

const (
	deckhousePath = "/deckhouse/"

	// magic number to limit count of concurrent parses. Way to avoid CPU throttling if it would be huge amount of files
	parserConcurrentCount = 50
)

type fileValidation struct {
	filePath string

	validationError error
}

type moduleVersions struct {
	specVersion        int
	conversionsVersion int
}

func modulesVersions(rootPath string) (map[string]*moduleVersions, error) {
	result := map[string]*moduleVersions{}
	err := filepath.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(path, "/openapi/") {
			var module string
			splits := strings.Split(path, "/")
			for i := 1; i < len(splits); i++ {
				if splits[i] == "openapi" && i > 0 {
					module = splits[i-1]
				}
			}
			if module == "" {
				return nil
			}
			if strings.HasSuffix(path, "config-values.yaml") {
				config := getFileYAMLContent(path)
				if val, ok := config["x-config-version"]; ok && val.(int) > 1 {
					if mv, ok := result[module]; ok {
						mv.specVersion = val.(int)
					} else {
						result[module] = &moduleVersions{specVersion: val.(int)}
					}
				}
			}
			if strings.Contains(path, "/conversions/") {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				var parsed struct {
					Version int
				}
				if err = yaml.Unmarshal(data, &parsed); err != nil {
					return err
				}
				if mv, ok := result[module]; ok {
					if parsed.Version > mv.conversionsVersion {
						mv.conversionsVersion = parsed.Version
					}
				} else {
					result[module] = &moduleVersions{conversionsVersion: parsed.Version}
				}
			}
		}
		return nil
	})
	return result, err
}

// GetOpenAPIYAMLFiles returns all .yaml files which are placed into openapi/ | crds/ directory
func GetOpenAPIYAMLFiles(rootPath string) ([]string, error) {
	var result []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// ignore matrix test specs
		if strings.HasSuffix(path, "-tests.yaml") {
			return nil
		}

		if strings.HasPrefix(info.Name(), "doc-ru-") {
			return nil
		}

		arr := strings.Split(path, "/")

		parentDir := arr[len(arr)-2]

		// check only openapi and crds specs
		switch parentDir {
		case "openapi", "crds":
		// pass

		default:
			return nil
		}

		result = append(result, path)

		return nil
	})

	return result, err
}

// RunOpenAPIValidator runs validator, get channel with file paths and returns channel with results
// nolint: revive // its a private lib, we dont need an exported struct
func RunOpenAPIValidator(fileC chan fileValidation) chan fileValidation {
	resultC := make(chan fileValidation, 1)

	go func() {
		for vfile := range fileC {
			parseResultC := make(chan error, parserConcurrentCount)

			yamlStruct := getFileYAMLContent(vfile.filePath)

			runFileParser(strings.TrimPrefix(vfile.filePath, deckhousePath), yamlStruct, parseResultC)

			var result *multierror.Error

			for err := range parseResultC {
				if err != nil {
					result = multierror.Append(result, err)
				}
			}

			resultErr := result.ErrorOrNil()
			if resultErr == nil {
				continue
			}
			resultC <- fileValidation{
				filePath:        vfile.filePath,
				validationError: resultErr,
			}
		}

		close(resultC)
	}()

	return resultC
}

type fileParser struct {
	fileName      string
	keyValidators map[string]validator

	resultC chan error
}

func getFileYAMLContent(path string) map[interface{}]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	m := make(map[interface{}]interface{})

	err = yaml.Unmarshal(data, &m)
	if err != nil {
		panic(err)
	}

	return m
}

func isCRD(data map[interface{}]interface{}) bool {
	kind, ok := data["kind"].(string)
	if !ok {
		return false
	}

	if kind != "CustomResourceDefinition" {
		return false
	}

	return true
}

func isDechkouseCRD(data map[interface{}]interface{}) bool {
	kind, ok := data["kind"].(string)
	if !ok {
		return false
	}

	if kind != "CustomResourceDefinition" {
		return false
	}

	metadata, ok := data["metadata"].(map[interface{}]interface{})
	if !ok {
		return false
	}

	name, ok := metadata["name"].(string)
	if !ok {
		return false
	}

	if strings.HasSuffix(name, "deckhouse.io") {
		return true
	}

	return false
}

func (fp fileParser) parseForWrongKeys(m map[interface{}]interface{}) {
	keysValidator := validators.NewKeyNameValidator()
	err := keysValidator.Run(fp.fileName, "allfile", m)
	if err != nil {
		fp.resultC <- err
	}
}

func runFileParser(fileName string, data map[interface{}]interface{}, resultC chan error) {
	// exclude external CRDs
	if isCRD(data) && !isDechkouseCRD(data) {
		close(resultC)
		return
	}

	fileParser := fileParser{
		fileName: fileName,
		keyValidators: map[string]validator{
			"enum":             validators.NewEnumValidator(),
			"highAvailability": validators.NewHAValidator(),
			"https":            validators.NewHAValidator(),
		},
		resultC: resultC,
	}
	if isDechkouseCRD(data) {
		fileParser.parseForWrongKeys(data)
	}
	go fileParser.startParsing(data, resultC)
}

func (fp fileParser) startParsing(m map[interface{}]interface{}, resultC chan error) {
	for k, v := range m {
		fp.parseValue(k.(string), v)
	}

	close(resultC)
}

func (fp fileParser) parseMap(upperKey string, m map[interface{}]interface{}) {
	for k, v := range m {
		absKey := fmt.Sprintf("%s.%s", upperKey, k)
		if key, ok := k.(string); ok {
			if validator, ok := fp.keyValidators[key]; ok {
				err := validator.Run(fp.fileName, absKey, v)
				if err != nil {
					fp.resultC <- err
				}
			}
		}
		fp.parseValue(absKey, v)
	}
}

func (fp fileParser) parseSlice(upperKey string, slc []interface{}) {
	for k, v := range slc {
		fp.parseValue(fmt.Sprintf("%s[%d]", upperKey, k), v)
	}
}

func (fp fileParser) parseValue(upperKey string, v interface{}) {
	if v == nil {
		return
	}
	typ := reflect.TypeOf(v).Kind()

	switch typ {
	case reflect.Map:
		fp.parseMap(upperKey, v.(map[interface{}]interface{}))
	case reflect.Slice:
		fp.parseSlice(upperKey, v.([]interface{}))
	}
}

type validator interface {
	Run(fileName, absoulteKey string, value interface{}) error
}
