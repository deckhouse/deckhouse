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
	candiDir = "/deckhouse/candi"

	providerSchemaFilenameSuffix = "_configuration.yaml"
)

func ParseConfig(path string) (*MetaConfig, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading schema file: %v", err)
	}

	return ParseConfigFromData(string(fileContent))
}

func ParseConfigFromData(configData string) (*MetaConfig, error) {
	schemaStore := NewSchemaStore()

	if err := filepath.Walk(candiDir, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, providerSchemaFilenameSuffix) {
			uploadError := schemaStore.UploadByPath(path)
			if uploadError != nil {
				return uploadError
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}

	bigFileTmp := strings.TrimSpace(configData)
	docs := regexp.MustCompile(`(?:^|\s*\n)---\s*`).Split(bigFileTmp, -1)

	metaConfig := MetaConfig{}
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		docData := []byte(doc)

		index, err := schemaStore.Validate(&docData)
		if err != nil {
			return nil, fmt.Errorf("config validation: %v", err)
		}

		var data map[string]json.RawMessage
		if err = yaml.Unmarshal(docData, &data); err != nil {
			return nil, fmt.Errorf("config unmarshal: %v", err)
		}

		switch {
		case index.Kind == "InitConfiguration":
			metaConfig.InitClusterConfig = data
		case index.Kind == "ClusterConfiguration":
			metaConfig.ClusterConfig = data
		case strings.HasSuffix(index.Kind, "ClusterConfiguration"):
			metaConfig.ProviderClusterConfig = data
		case strings.HasSuffix(index.Kind, "InitConfiguration"):
			metaConfig.InitProviderClusterConfig = data
		}
	}

	metaConfig.Prepare()
	return &metaConfig, nil
}
