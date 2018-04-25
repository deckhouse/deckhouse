package utils

import (
	"fmt"
	"github.com/go-yaml/yaml"
)

// convert yaml structure to string
// Can be used for error formating:
// fmt.Errorf("expected map at key 'global', got:\n%s", utils.YamlToString(globalValuesRaw))
//
// !! Panic if data not converted to yaml to speed up detecting problems. !!
func YamlToString(data interface{}) string {
	valuesYaml, err := yaml.Marshal(&data)
	if err != nil {
		panic(fmt.Sprintf("Cannot dump data to yaml: \n%#v\n error: %s", data, err))
	}
	return string(valuesYaml)
}
