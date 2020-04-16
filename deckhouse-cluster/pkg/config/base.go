package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	providerSchemaFilename = "provider-schema.yaml"
)

var sep = regexp.MustCompile(`(?:^|\s*\n)---\s*`)

func ParseConfig(path string) (*MetaConfig, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading schema file: %v", err)
	}

	return ParseConfigFromData(string(fileContent))
}

func ParseConfigFromData(configData string) (*MetaConfig, error) {
	schemaStore := NewSchemaStore()

	err := filepath.Walk(os.Getenv("MODULES_DIR"), func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, providerSchemaFilename) {
			uploadError := schemaStore.UploadByPath(path)
			if uploadError != nil {
				return uploadError
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("filepath walk error: %v", err)
	}

	bigFileTmp := strings.TrimSpace(configData)
	docs := sep.Split(bigFileTmp, -1)

	var clusterConfig map[string]json.RawMessage
	var providerConfig map[string]json.RawMessage

	for _, d := range docs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}

		docData := []byte(d)

		index, err := schemaStore.Validate(&docData)
		if err != nil {
			return nil, err
		}

		if index.Kind == "ClusterConfiguration" {
			err = yaml.Unmarshal(docData, &clusterConfig)
		} else {
			err = yaml.Unmarshal(docData, &providerConfig)
		}

		if err != nil {
			return nil, err
		}
	}

	return NewMetaConfig(clusterConfig, providerConfig), nil
}
