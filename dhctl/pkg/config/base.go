// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	candiDir                 = "/deckhouse/candi"
	modulesDir               = "/deckhouse/modules"
	globalHooksModule        = "/deckhouse/global-hooks"
	DefaultKubernetesVersion = "1.27"
)

const (
	versionMap        = "/deckhouse/candi/version_map.yml"
	imagesDigestsJSON = "/deckhouse/candi/images_digests.json"
)

func LoadConfigFromFile(path string) (*MetaConfig, error) {
	metaConfig, err := ParseConfig(path)
	if err != nil {
		return nil, err
	}

	if metaConfig.ClusterConfig == nil {
		return nil, fmt.Errorf("ClusterConfiguration must be provided")
	}

	err = metaConfig.LoadVersionMap(versionMap)
	if err != nil {
		return nil, err
	}

	err = metaConfig.LoadImagesDigests(imagesDigestsJSON)
	if err != nil {
		return nil, err
	}

	err = metaConfig.LoadInstallerVersion()
	if err != nil {
		return nil, err
	}

	if metaConfig.ClusterType == CloudClusterType && len(metaConfig.ProviderClusterConfig) == 0 {
		return nil, fmt.Errorf("ProviderClusterConfiguration section is required for a Cloud cluster.")
	}

	return metaConfig, nil
}

func numerateManifestLines(manifest []byte) string {
	manifestLines := strings.Split(string(manifest), "\n")
	builder := strings.Builder{}

	for index, line := range manifestLines {
		builder.WriteString(fmt.Sprintf("%d\t%s\n", index+1, line))
	}

	return builder.String()
}

func ParseConfig(path string) (*MetaConfig, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading config file: %v", err)
	}

	return ParseConfigFromData(string(fileContent))
}

func ParseConfigFromCluster(kubeCl *client.KubernetesClient) (*MetaConfig, error) {
	var metaConfig *MetaConfig
	var err error
	err = log.Process("common", "Get Cluster configuration", func() error {
		return retry.NewLoop("Get Cluster configuration from Kubernetes cluster", 10, 5*time.Second).Run(func() error {
			metaConfig, err = parseConfigFromCluster(kubeCl)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	return metaConfig, nil
}

func ParseConfigInCluster(kubeCl *client.KubernetesClient) (*MetaConfig, error) {
	var metaConfig *MetaConfig
	var err error

	err = retry.NewSilentLoop("Get Cluster configuration from inside Kubernetes cluster", 5, 5*time.Second).Run(func() error {
		metaConfig, err = parseConfigFromCluster(kubeCl)
		return err
	})
	if err != nil {
		return nil, err
	}
	return metaConfig, nil
}

func parseConfigFromCluster(kubeCl *client.KubernetesClient) (*MetaConfig, error) {
	metaConfig := MetaConfig{}
	schemaStore := NewSchemaStore()

	clusterConfig, err := kubeCl.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	clusterConfigData := clusterConfig.Data["cluster-configuration.yaml"]
	_, err = schemaStore.Validate(&clusterConfigData)
	if err != nil {
		return nil, err
	}

	var parsedClusterConfig map[string]json.RawMessage
	if err := yaml.Unmarshal(clusterConfigData, &parsedClusterConfig); err != nil {
		return nil, err
	}

	metaConfig.ClusterConfig = parsedClusterConfig

	var clusterType string
	if err := json.Unmarshal(parsedClusterConfig["clusterType"], &clusterType); err != nil {
		return nil, err
	}

	if clusterType == CloudClusterType {
		providerClusterConfig, err := kubeCl.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-provider-cluster-configuration", metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		providerClusterConfigData := providerClusterConfig.Data["cloud-provider-cluster-configuration.yaml"]
		_, err = schemaStore.Validate(&providerClusterConfigData)
		if err != nil {
			return nil, err
		}

		var parsedProviderClusterConfig map[string]json.RawMessage
		if err := yaml.Unmarshal(providerClusterConfigData, &parsedProviderClusterConfig); err != nil {
			return nil, err
		}

		metaConfig.ProviderClusterConfig = parsedProviderClusterConfig
	}

	return metaConfig.Prepare()
}

func parseDocument(doc string, metaConfig *MetaConfig, schemaStore *SchemaStore) error {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return nil
	}

	docData := []byte(doc)

	var index SchemaIndex
	err := yaml.Unmarshal(docData, &index)
	if err != nil {
		return err
	}

	if index.Kind == ModuleConfigKind {
		moduleConfig := ModuleConfig{}
		err = yaml.Unmarshal(docData, &moduleConfig)
		if err != nil {
			return err
		}

		_, err = schemaStore.Validate(&docData)
		if err != nil {
			return fmt.Errorf("module config validation: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
		}

		metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, &moduleConfig)
		return nil
	}

	_, err = schemaStore.Validate(&docData)
	if err != nil {
		return fmt.Errorf("config validation: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
	}

	var data map[string]json.RawMessage
	if err = yaml.Unmarshal(docData, &data); err != nil {
		return fmt.Errorf("config unmarshal: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
	}

	switch {
	case index.Kind == "InitConfiguration":
		metaConfig.InitClusterConfig = data
	case index.Kind == "ClusterConfiguration":
		metaConfig.ClusterConfig = data
	case index.Kind == "StaticClusterConfiguration":
		metaConfig.StaticClusterConfig = data
	case strings.HasSuffix(index.Kind, "ClusterConfiguration"):
		metaConfig.ProviderClusterConfig = data
	}

	return nil
}

func ParseConfigFromData(configData string) (*MetaConfig, error) {
	schemaStore := NewSchemaStore()

	bigFileTmp := strings.TrimSpace(configData)
	docs := input.YAMLSplitRegexp.Split(bigFileTmp, -1)

	metaConfig := MetaConfig{}
	for _, doc := range docs {
		if err := parseDocument(doc, &metaConfig, schemaStore); err != nil {
			return nil, err
		}
	}

	// init configuration can be empty, but we need default from openapi spec
	if len(metaConfig.InitClusterConfig) == 0 {
		doc := `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse: {}
`
		if err := parseDocument(doc, &metaConfig, schemaStore); err != nil {
			return nil, err
		}
	}

	return metaConfig.Prepare()
}

// ValidateClusterSettings parses and validates cluster configuration and resources.
// It checks the cluster configuration yamls for compliance with the yaml format and schema.
// Non-config resources are checked only for compliance with the yaml format and the validity of apiVersion and kind fields.
// It can be used as an imported functionality in external modules.
func ValidateClusterSettings(configData string) error {
	schemaStore := NewSchemaStore()

	bigFileTmp := strings.TrimSpace(configData)
	docs := input.YAMLSplitRegexp.Split(bigFileTmp, -1)

	metaConfig := MetaConfig{}
	for _, doc := range docs {
		err := parseDocument(doc, &metaConfig, schemaStore)
		// Cluster resources are not stored in the dhctl cache, there is no need to check them for compliance with the schema: just check the index and yaml format.
		if err != nil && !errors.Is(err, ErrSchemaNotFound) {
			return err
		}
	}

	_, err := metaConfig.Prepare()
	if err != nil {
		return err
	}

	return nil
}
