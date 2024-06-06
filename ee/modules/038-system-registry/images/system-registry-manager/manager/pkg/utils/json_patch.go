/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch"
	"sigs.k8s.io/yaml"
)

// PatchOperation represents a single patch operation.
type PatchOperation struct {
	Op    string  `yaml:"op" json:"op"`
	Path  string  `yaml:"path" json:"path"`
	Value *string `yaml:"value" json:"value"`
}

// NewPatchRemove creates a new PatchOperation to remove a JSON/YAML field.
func NewPatchRemove(path string) PatchOperation {
	return PatchOperation{
		Op:   "remove",
		Path: path,
	}
}

// NewPatchReplace creates a new PatchOperation to replace a JSON/YAML field with a value.
func NewPatchReplace(path string, value string) PatchOperation {
	return PatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

// ApplyPatchForYaml applies patch operations to YAML content.
func ApplyPatchForYaml(yamlContent string, patchOperations []PatchOperation) (string, error) {
	jsonContent, err := yaml.YAMLToJSON([]byte(yamlContent))
	if err != nil {
		return "", fmt.Errorf("error converting YAML to JSON: %v", err)
	}

	jsonContentWithPatch, err := ApplyPatchForJson(string(jsonContent), patchOperations)
	if err != nil {
		return "", err
	}

	yamlContentWithPatch, err := yaml.JSONToYAML([]byte(jsonContentWithPatch))
	if err != nil {
		return "", fmt.Errorf("error converting JSON to YAML: %v", err)
	}

	return string(yamlContentWithPatch), nil
}

// ApplyPatchForJson applies patch operations to JSON content.
func ApplyPatchForJson(jsonContent string, patchOperations []PatchOperation) (string, error) {
	jsonPatchFormat, err := json.Marshal(patchOperations)
	if err != nil {
		return "", fmt.Errorf("error marshalling patch operations to JSON: %v", err)
	}

	patch, err := jsonpatch.DecodePatch(jsonPatchFormat)
	if err != nil {
		return "", fmt.Errorf("error decoding JSON patch: %v", err)
	}

	modified, err := patch.Apply([]byte(jsonContent))
	if err != nil {
		return "", fmt.Errorf("error applying JSON patch: %v", err)
	}

	return string(modified), nil
}
