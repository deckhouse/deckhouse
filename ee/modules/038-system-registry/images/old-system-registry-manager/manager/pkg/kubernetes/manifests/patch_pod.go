/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package manifests

import (
	"fmt"
	"gopkg.in/yaml.v2"
)

func ChangePodAnnotations(manifest []byte, newAnnotations map[string]string) ([]byte, error) {
	var data map[string]interface{}

	// Unmarshal the YAML manifest
	if err := yaml.Unmarshal(manifest, &data); err != nil {
		return nil, fmt.Errorf("error parsing YAML: %v", err)
	}

	// Ensure that the metadata field exists
	metadata, ok := data["metadata"].(map[interface{}]interface{})
	if !ok {
		metadata = make(map[interface{}]interface{})
		data["metadata"] = metadata
	}

	// Ensure that the annotations field exists
	annotations, ok := metadata["annotations"].(map[interface{}]interface{})
	if !ok {
		annotations = make(map[interface{}]interface{})
		metadata["annotations"] = annotations
	}

	// Update the annotations
	for key, value := range newAnnotations {
		annotations[key] = value
	}

	// Marshal the data structure back into YAML
	modifiedManifest, err := yaml.Marshal(&data)
	if err != nil {
		return nil, fmt.Errorf("error serializing to YAML: %v", err)
	}

	return modifiedManifest, nil
}
