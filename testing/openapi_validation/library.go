/*
Copyright 2021 Flant CJSC

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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	deckhousePath = "/deckhouse/"
	// magic number to limit count of concurrent parses. Way to avoid CPU throttling if it would be huge amount of files
	parserConcurrentCount = 50
)

var (
	// openapi key excludes by file
	fileExcludes = map[string][]string{
		// all files
		"*": {"apiVersions[*].openAPISpec.properties.apiVersion"},
		// exclude zone - ru-center-1, ru-center-2, ru-center-3
		"candi/cloud-providers/yandex/openapi/cluster_configuration.yaml": {
			"apiVersions[0].openAPISpec.properties.nodeGroups.items.properties.zones.items",
			"apiVersions[0].openAPISpec.properties.masterNodeGroup.properties.zones.items",
			"apiVersions[0].openAPISpec.properties.zones.items",
		},
		// disk types - gp2.,..
		"candi/cloud-providers/aws/openapi/cluster_configuration.yaml": {
			"apiVersions[0].openAPISpec.properties.masterNodeGroup.properties.instanceClass.properties.diskType",
			"apiVersions[0].openAPISpec.properties.nodeGroups.items.properties.instanceClass.properties.diskType",
		},
		// disk types: pd-standard, pd-ssd, ...
		"candi/cloud-providers/gcp/openapi/instance_class.yaml": {
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.diskType",
		},
		// disk types: network-ssd, network-hdd
		"candi/cloud-providers/yandex/openapi/instance_class.yaml": {
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.diskType",
			// v1alpha1 : SOFTWARE_ACCELERATED - migrated in v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.networkType",
		},
		"candi/openapi/cluster_configuration.yaml": {
			// vSphere
			"apiVersions[0].openAPISpec.properties.cloud.properties.provider",
		},
		"modules/010-user-authn-crd/crds/dex-provider.yaml": {
			// v1alpha1 migrated to v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.github.properties.teamNameField",
		},
		"modules/010-prometheus-crd/crds/grafanaadditionaldatasources.yaml": {
			// v1alpha1 migrated to v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.access",
		},
		"modules/035-cni-flannel/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.podNetworkMode",
		},
		"modules/042-kube-dns/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.specificNodeType",
		},
		"modules/402-ingress-nginx/crds/ingress-nginx.yaml": {
			// GeoIP base constants: GeoIP2-ISP, GeoIP2-ASN, ...
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.geoIP2.properties.maxmindEditionIDs.items",
		},
	}
)

var (
	arrayPathRegex = regexp.MustCompile(`\[\d+\]`)
)

type fileValidation struct {
	filePath string

	enumErr error
}

// GetOpenAPIYAMLFiles returns all .yaml files which are placed into openapi/ directory
func GetOpenAPIYAMLFiles(rootPath string) ([]string, error) {
	var result []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".yaml") {
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
// nolint: golint // its a private lib, we dont need an exported struct
func RunOpenAPIValidator(fileC chan fileValidation) chan fileValidation {
	resultC := make(chan fileValidation, 1)

	go func() {
		for vfile := range fileC {
			enumParseResultC := make(chan map[string][]string, parserConcurrentCount)

			yamlStruct := getFileYAMLContent(vfile.filePath)

			runEnumParser(strings.TrimPrefix(vfile.filePath, deckhousePath), yamlStruct, enumParseResultC)

			var result *multierror.Error

			for res := range enumParseResultC {
				for enumKey, values := range res {
					err := validateEnumValues(enumKey, values)
					if err != nil {
						result = multierror.Append(result, err)
					}
				}
			}

			resultC <- fileValidation{
				filePath: vfile.filePath,
				enumErr:  result.ErrorOrNil(),
			}
		}

		close(resultC)
	}()

	return resultC
}

type fileParser struct {
	excludeKeys map[string]struct{}

	resultC chan map[string][]string
}

func getFileYAMLContent(path string) map[interface{}]interface{} {
	data, err := ioutil.ReadFile(path)
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

func runEnumParser(fileName string, data map[interface{}]interface{}, resultC chan map[string][]string) {
	// exclude external CRDs
	if !isDechkouseCRD(data) {
		close(resultC)
		return
	}

	keyExcludes := make(map[string]struct{})

	for _, exc := range fileExcludes["*"] {
		keyExcludes[exc+".enum"] = struct{}{}
	}

	for _, exc := range fileExcludes[fileName] {
		keyExcludes[exc+".enum"] = struct{}{}
	}

	fileParser := fileParser{
		excludeKeys: keyExcludes,
		resultC:     resultC,
	}

	go fileParser.startParsing(data, resultC)
}

func validateEnumValues(enumKey string, values []string) *multierror.Error {
	var res *multierror.Error
	for _, value := range values {
		err := validateEnumValue(value)
		if err != nil {
			res = multierror.Append(res, errors.Wrap(err, fmt.Sprintf("Enum '%s' is invalid", enumKey)))
		}
	}

	return res
}

func validateEnumValue(value string) error {
	if len(value) == 0 {
		return nil
	}

	vv := []rune(value)
	if (vv[0] < 'A' || vv[0] > 'Z') && (vv[0] < '0' || vv[0] > '9') {
		return fmt.Errorf("value '%s' must start with Capital letter", value)
	}

	if strings.ContainsAny(value, " -_") {
		return fmt.Errorf("value: '%s' must be in CamelCase", value)
	}

	return nil
}

func (fp fileParser) startParsing(m map[interface{}]interface{}, resultC chan map[string][]string) {
	for k, v := range m {
		fp.parseValue(k.(string), v)
	}

	close(resultC)
}

func (fp fileParser) parseMap(upperKey string, m map[interface{}]interface{}) {
	for k, v := range m {
		absKey := fmt.Sprintf("%s.%s", upperKey, k)
		if k == "enum" {
			if _, ok := fp.excludeKeys[absKey]; ok {
				// excluding key, dont check it
				continue
			}

			// check for slice path with wildcard
			index := arrayPathRegex.FindString(absKey)
			if index != "" {
				wildcardKey := strings.ReplaceAll(absKey, index, "[*]")
				if _, ok := fp.excludeKeys[wildcardKey]; ok {
					// excluding key with wildcard
					continue
				}
			}

			values := v.([]interface{})
			enum := make([]string, 0, len(values))
			for _, val := range values {
				valStr, ok := val.(string)
				if !ok {
					continue // skip boolean flags
				}
				enum = append(enum, valStr)
			}
			fp.resultC <- map[string][]string{absKey: enum}
			continue
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
