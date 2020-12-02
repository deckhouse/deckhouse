package helm

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

func ConvertYAMLToJSON(yamlBytes []byte) ([]byte, error) {
	var obj interface{}

	err := yaml.Unmarshal(yamlBytes, &obj)
	if err != nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return jsonBytes, nil
}

func ConvertJSONToYAML(jsonBytes []byte) ([]byte, error) {
	var obj interface{}

	err := json.Unmarshal(jsonBytes, &obj)
	if err != nil {
		return nil, err
	}

	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return yamlBytes, nil
}
