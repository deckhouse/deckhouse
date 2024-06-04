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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	candiDir          = "./deckhouse/candi"
	modulesDir        = "./deckhouse/modules"
	globalHooksModule = "./deckhouse/global-hooks"
	// don't forget to update the version in release requirements (release.yaml) 'autoK8sVersion' key
	DefaultKubernetesVersion = "1.27"
)

const (
	versionMap        = "./deckhouse/candi/version_map.yml"
	imagesDigestsJSON = "./deckhouse/candi/images_digests.json"
)

func LoadConfigFromFile(paths []string, opts ...ValidateOption) (*MetaConfig, error) {
	metaConfig, err := ParseConfig(paths, opts...)
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

func ParseConfig(paths []string, opts ...ValidateOption) (*MetaConfig, error) {
	content := ""
	for _, path := range paths {
		log.DebugF("Have config file %s\n", path)
		fileContent, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading config file: %v", err)
		}
		content = content + "\n\n---\n\n" + string(fileContent)
	}

	return ParseConfigFromData(content, opts...)
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

// parseDocument
//
// parse and validate one document of
//
//		InitConfiguration
//		ClusterConfiguration
//		StaticClusterConfiguration
//		ClusterConfiguration
//	    ModuleConfig
//
// if validation schema for ModuleConfig or another resources not found returns ErrSchemaNotFound error
func parseDocument(doc string, metaConfig *MetaConfig, schemaStore *SchemaStore, opts ...ValidateOption) (bool, error) {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return false, nil
	}

	docData := []byte(doc)

	var index SchemaIndex
	err := yaml.Unmarshal(docData, &index)
	if err != nil {
		return false, fmt.Errorf("Config document unmarshal failed: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
	}

	if index.Kind == ModuleConfigKind {
		moduleConfig := ModuleConfig{}
		err = yaml.Unmarshal(docData, &moduleConfig)
		if err != nil {
			return false, fmt.Errorf("Module config document unmarshal failed: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
		}

		log.DebugF("Found ModuleConfig in config file %s\n", moduleConfig.Name)

		_, err = schemaStore.Validate(&docData, opts...)
		if err != nil {
			if errors.Is(err, ErrSchemaNotFound) {
				return false, nil
			}
			return false, fmt.Errorf("Module config validation failed: %w\ndata: \n%s\n", err, numerateManifestLines(docData))
		}

		metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, &moduleConfig)
		return true, nil
	}

	_, err = schemaStore.Validate(&docData, opts...)
	if err != nil {
		if errors.Is(err, ErrSchemaNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("Config document validation failed: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
	}

	var data map[string]json.RawMessage
	if err = yaml.Unmarshal(docData, &data); err != nil {
		return false, fmt.Errorf("Config document unmarshal failed: %v\ndata: \n%s\n", err, numerateManifestLines(docData))
	}

	found := false
	switch {
	case index.Kind == "InitConfiguration":
		log.DebugLn("Found InitConfiguration")
		metaConfig.InitClusterConfig = data
		found = true
	case index.Kind == "ClusterConfiguration":
		log.DebugLn("Found ClusterConfiguration")
		metaConfig.ClusterConfig = data
		found = true
	case index.Kind == "StaticClusterConfiguration":
		log.DebugLn("Found StaticClusterConfiguration")
		metaConfig.StaticClusterConfig = data
		found = true
	case strings.HasSuffix(index.Kind, "ClusterConfiguration"):
		log.DebugF("Found %s\n", index.Kind)
		metaConfig.ProviderClusterConfig = data
		found = true
	}

	return found, nil
}

func ParseConfigFromData(configData string, opts ...ValidateOption) (*MetaConfig, error) {
	schemaStore := NewSchemaStore()

	bigFileTmp := strings.TrimSpace(configData)
	docs := input.YAMLSplitRegexp.Split(bigFileTmp, -1)

	resourcesDocs := make([]string, 0, len(docs))

	metaConfig := MetaConfig{}
	for _, doc := range docs {
		found, err := parseDocument(doc, &metaConfig, schemaStore, opts...)
		if err != nil {
			return nil, err
		}
		if !found && strings.TrimSpace(doc) != "" {
			resourcesDocs = append(resourcesDocs, doc)
		}
	}

	metaConfig.ResourcesYAML = strings.TrimSpace(strings.Join(resourcesDocs, "\n\n---\n\n"))
	log.DebugF("Collected ResourcesYAML:\n%s\n\n", metaConfig.ResourcesYAML)

	// init configuration can be empty, but we need default from openapi spec
	if len(metaConfig.InitClusterConfig) == 0 {
		doc := `
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse: {}
`
		log.DebugF("Init configuration not found use empty: %s", doc)
		found, err := parseDocument(doc, &metaConfig, schemaStore, opts...)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("init configuration index not found")
		}
	}

	return metaConfig.Prepare()
}
