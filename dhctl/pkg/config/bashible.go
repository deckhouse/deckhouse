package config

import (
	"fmt"
	"io/ioutil"

	"sigs.k8s.io/yaml"
)

func ParseBashibleConfig(path, specPath string) (map[string]interface{}, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading schema file: %v", err)
	}

	schemaStore := NewSchemaStore()
	err = schemaStore.UploadByPath(specPath)
	if err != nil {
		return nil, fmt.Errorf("loading bashible schema: %v", err)
	}

	_, err = schemaStore.Validate(&fileContent)
	if err != nil {
		return nil, fmt.Errorf("config validation: %v", err)
	}

	var data map[string]interface{}
	if err = yaml.Unmarshal(fileContent, &data); err != nil {
		return nil, fmt.Errorf("config unmarshal: %v", err)
	}

	return data, nil
}
