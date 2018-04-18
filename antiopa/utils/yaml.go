package utils

import (
	"fmt"
	"github.com/go-yaml/yaml"
)

func YamlToString(data interface{}) string {
	valuesYaml, err := yaml.Marshal(&data)
	if err != nil {
		panic(fmt.Sprintf("Cannot dump data to yaml: %s\n%#v error: %s", data, err))
	}
	return string(valuesYaml)
}
