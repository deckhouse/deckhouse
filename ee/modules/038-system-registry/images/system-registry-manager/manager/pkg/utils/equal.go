/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"fmt"
	jsonpatch "github.com/evanphx/json-patch"
	"sigs.k8s.io/yaml"
)

func IsValidYAML(yamlContent string) bool {
	var temp map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlContent), &temp)
	return err == nil
}

func EqualYaml(lYamlContent, rYamlContent string) (bool, error) {
	if !IsValidYAML(lYamlContent) {
		return false, fmt.Errorf("left YAML content is not valid")
	}
	if !IsValidYAML(rYamlContent) {
		return false, fmt.Errorf("right YAML content is not valid")
	}

	lJsonContent, err := yaml.YAMLToJSON([]byte(lYamlContent))
	if err != nil {
		return false, fmt.Errorf("error converting left YAML to JSON: %v", err)
	}
	rJsonContent, err := yaml.YAMLToJSON([]byte(rYamlContent))
	if err != nil {
		return false, fmt.Errorf("error converting right YAML to JSON: %v", err)
	}

	isEqual := EqualJson(string(lJsonContent), string(rJsonContent))
	return isEqual, nil
}

func EqualJson(lJsonContent, rJsonContent string) bool {
	return jsonpatch.Equal([]byte(lJsonContent), []byte(rJsonContent))
}
