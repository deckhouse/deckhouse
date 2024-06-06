/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package utils

import (
	"testing"
)

func TestApplyPatchForYaml(t *testing.T) {
	tests := []struct {
		name             string
		yamlContent      string
		patchOperations  []PatchOperation
		expectedYaml     string
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name:        "Replace operation",
			yamlContent: "foo: bar\n",
			patchOperations: []PatchOperation{
				NewPatchReplace("/foo", "baz"),
			},
			expectedYaml: "foo: baz\n",
		},
		{
			name:        "Remove operation",
			yamlContent: "foo: bar\n",
			patchOperations: []PatchOperation{
				NewPatchRemove("/foo"),
			},
			expectedYaml: "{}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualYaml, err := ApplyPatchForYaml(tt.yamlContent, tt.patchOperations)

			if tt.expectedError {
				if err == nil || err.Error() != tt.expectedErrorMsg {
					t.Errorf("Expected error '%s', got '%v'", tt.expectedErrorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if actualYaml != tt.expectedYaml {
					t.Errorf("Expected YAML: '%s', got: '%s'", tt.expectedYaml, actualYaml)
				}
			}
		})
	}
}

func TestApplyPatchForJson(t *testing.T) {
	tests := []struct {
		name             string
		jsonContent      string
		patchOperations  []PatchOperation
		expectedJson     string
		expectedError    bool
		expectedErrorMsg string
	}{
		{
			name:        "Replace operation",
			jsonContent: `{"foo":"bar"}`,
			patchOperations: []PatchOperation{
				NewPatchReplace("/foo", "baz"),
			},
			expectedJson: `{"foo":"baz"}`,
		},
		{
			name:        "Remove operation",
			jsonContent: `{"foo":"bar"}`,
			patchOperations: []PatchOperation{
				NewPatchRemove("/foo"),
			},
			expectedJson: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualJson, err := ApplyPatchForJson(tt.jsonContent, tt.patchOperations)

			if tt.expectedError {
				if err == nil || err.Error() != tt.expectedErrorMsg {
					t.Errorf("Expected error '%s', got '%v'", tt.expectedErrorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if actualJson != tt.expectedJson {
					t.Errorf("Expected JSON: '%s', got: '%s'", tt.expectedJson, actualJson)
				}
			}
		})
	}
}
