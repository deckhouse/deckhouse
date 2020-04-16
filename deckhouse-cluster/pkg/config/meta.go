package config

import (
	"encoding/json"
	"strings"

	"github.com/peterbourgon/mergemap"
	"sigs.k8s.io/yaml"
)

type MetaConfig struct {
	ClusterType  string `json:"-"`
	ProviderName string `json:"-"`
	Layout       string `json:"-"`

	ClusterConfig         map[string]json.RawMessage `json:"clusterConfig"`
	ProviderClusterConfig map[string]json.RawMessage `json:"providerClusterConfig"`

	BootstrapConfig *BootstrapClusterConfig `json:"-"`
}

func NewMetaConfig(cc, pc map[string]json.RawMessage) *MetaConfig {
	metaConfig := MetaConfig{ClusterConfig: cc, ProviderClusterConfig: pc}

	var clusterConfigSpec ClusterConfigSpec
	_ = json.Unmarshal(cc["spec"], &clusterConfigSpec)

	metaConfig.ClusterType = clusterConfigSpec.ClusterType

	if metaConfig.ClusterType == "Cloud" {
		metaConfig.ProviderName = strings.ToLower(clusterConfigSpec.Cloud["provider"].(string))

		var providerConfigSpec ProviderClusterConfigSpec
		_ = json.Unmarshal(pc["spec"], &providerConfigSpec)

		metaConfig.Layout = strings.ToLower(providerConfigSpec.Layout)
	}

	return &metaConfig
}

func (m *MetaConfig) PrepareBootstrapSettings() {
	var bootstrapConfig BootstrapClusterConfig
	_ = json.Unmarshal(m.ClusterConfig["bootstrap"], &bootstrapConfig)

	m.BootstrapConfig = &bootstrapConfig
}

func (m *MetaConfig) MergeDeckhouseConfig(configs ...[]byte) map[string]interface{} {
	var firstConfig map[string]interface{}
	_ = json.Unmarshal(configs[0], &firstConfig)

	for _, configRaw := range configs[1:] {
		var config map[string]interface{}
		_ = json.Unmarshal(configRaw, &config)

		firstConfig = mergemap.Merge(firstConfig, config)
	}

	firstConfig = mergemap.Merge(firstConfig, m.BootstrapConfig.Deckhouse.ConfigOverrides)
	firstConfig = mergemap.Merge(firstConfig, map[string]interface{}{
		"deckhouse": map[string]interface{}{
			"logLevel":       m.BootstrapConfig.Deckhouse.LogLevel,
			"bundle":         m.BootstrapConfig.Deckhouse.Bundle,
			"releaseChannel": m.BootstrapConfig.Deckhouse.ReleaseChannel,
		},
	})

	return firstConfig
}

func (m *MetaConfig) MergeNodeGroupConfig(instanceClass []byte) ([]byte, error) {
	var doc map[string]json.RawMessage

	err := json.Unmarshal(instanceClass, &doc)
	if err != nil {
		return nil, err
	}

	var metadata struct{ Name string }
	err = json.Unmarshal(doc["metadata"], &metadata)
	if err != nil {
		return nil, err
	}

	nodeGroup := NodeGroup{Kind: "NodeGroup", APIVersion: "deckhouse.io/v1beta1", Spec: NodeGroupSpec{NodeType: "Cloud"}}
	for key, value := range m.BootstrapConfig.MasterNodeGroup {
		nodeGroup.Spec.CloudInstances[key] = value
	}

	nodeGroup.Spec.CloudInstances["classReference"] = ClassReference{Kind: string(doc["kind"]), Name: metadata.Name}

	return json.Marshal(nodeGroup)
}

func (m *MetaConfig) MarshalConfig(bootstrap bool) ([]byte, error) {
	if bootstrap {
		return json.Marshal(m)
	}
	return json.Marshal(MetaConfig{
		ClusterConfig:         filterMap(m.ClusterConfig),
		ProviderClusterConfig: filterMap(m.ProviderClusterConfig),
	})
}

func (m *MetaConfig) MarshalClusterConfigYAML() ([]byte, error) {
	return yaml.Marshal(filterMap(m.ClusterConfig))
}

func (m *MetaConfig) MarshalProviderClusterConfigYAML() ([]byte, error) {
	return yaml.Marshal(filterMap(m.ProviderClusterConfig))
}

func filterMap(config map[string]json.RawMessage) map[string]json.RawMessage {
	newMap := make(map[string]json.RawMessage, len(config))
	for key, value := range config {
		if key == "bootstrap" {
			continue
		}
		newMap[key] = value
	}
	return newMap
}
