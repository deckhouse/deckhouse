package config

import (
	"encoding/json"
	"flant/deckhouse-candi/pkg/kube"
	"fmt"
	"github.com/flant/logboek"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

func ParseConfigFromCluster(kubeCl *kube.KubernetesClient) (*MetaConfig, error) {
	for i := 1; i < 45; i++ {
		metaConfig, err := parseConfigFromCluster(kubeCl)
		if err == nil {
			return metaConfig, nil
		}

		logboek.LogInfoF("[Attempt #%v of 45] Getting cluster configuration failed, next attempt in 10s\n", i)
		logboek.LogWarnF("%v\n\n", err)

		time.Sleep(10 * time.Second)
	}
	return nil, fmt.Errorf("timeout while getting cluster configuration")
}

func parseConfigFromCluster(kubeCl *kube.KubernetesClient) (*MetaConfig, error) {
	metaConfig := MetaConfig{}

	clusterConfig, err := kubeCl.CoreV1().Secrets("kube-system").Get("d8-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var clusterConfigData map[string]json.RawMessage
	if err := yaml.Unmarshal(clusterConfig.Data["cluster-configuration.yaml"], &clusterConfigData); err != nil {
		return nil, err
	}

	metaConfig.ClusterConfig = clusterConfigData

	var clusterType string
	if err := json.Unmarshal(clusterConfigData["clusterType"], &clusterType); err != nil {
		return nil, err
	}

	if clusterType == CloudClusterType {
		providerClusterConfig, err := kubeCl.CoreV1().Secrets("kube-system").Get("d8-provider-cluster-configuration", metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		var providerClusterConfigData map[string]json.RawMessage
		if err := yaml.Unmarshal(providerClusterConfig.Data["cloud-provider-cluster-configuration.yaml"], &providerClusterConfigData); err != nil {
			return nil, err
		}

		metaConfig.ProviderClusterConfig = providerClusterConfigData
	}
	return &metaConfig, nil
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
