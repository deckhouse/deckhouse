package utils

import (
	"fmt"
	"github.com/go-yaml/yaml"
)

// convert yaml structure to string
// Can be used for error formating:
// fmt.Errorf("expected map at key 'global', got:\n%s", utils.YamlToString(globalValuesRaw))
func YamlToString(data interface{}) string {
	valuesYaml, err := yaml.Marshal(&data)
	if err != nil {
		return fmt.Sprintf("YAML error: %s>>>\n%#v\n>>>", err, data)
	}
	return string(valuesYaml)
}
