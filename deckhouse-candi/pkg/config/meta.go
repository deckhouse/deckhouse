package config

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/peterbourgon/mergemap"
	"sigs.k8s.io/yaml"
)

type MetaConfig struct {
	ClusterType           string `json:"-"`
	ProviderName          string `json:"-"`
	providerNameCamelCase string `json:"-"`
	Layout                string `json:"-"`

	DeckhouseConfig DeckhouseClusterConfig `json:"-"`

	ClusterConfig             map[string]json.RawMessage `json:"clusterConfig"`
	ProviderClusterConfig     map[string]json.RawMessage `json:"providerClusterConfig"`
	InitClusterConfig         map[string]json.RawMessage `json:"initConfig"`
	InitProviderClusterConfig map[string]json.RawMessage `json:"providerInitConfig"`
}

func (m *MetaConfig) Prepare() {
	_ = json.Unmarshal(m.ClusterConfig["clusterType"], &m.ClusterType)

	if m.ClusterType == "Cloud" {
		cloud := struct {
			Provider string `json:"provider"`
		}{}
		// Validated by openapi schema
		_ = json.Unmarshal(m.ClusterConfig["cloud"], &cloud)
		_ = json.Unmarshal(m.ProviderClusterConfig["layout"], &m.Layout)
		m.Layout = strings.ToLower(m.Layout)

		m.ProviderName = strings.ToLower(cloud.Provider)
		m.providerNameCamelCase = cloud.Provider
	}

	_ = json.Unmarshal(m.InitClusterConfig["deckhouse"], &m.DeckhouseConfig)
}

func (m *MetaConfig) MergeDeckhouseConfig(configs ...[]byte) map[string]interface{} {
	deckhouseModuleConfig := map[string]interface{}{
		"logLevel": m.DeckhouseConfig.LogLevel,
		"bundle":   m.DeckhouseConfig.Bundle,
	}

	if m.DeckhouseConfig.ReleaseChannel != "" {
		deckhouseModuleConfig["releaseChannel"] = m.DeckhouseConfig.ReleaseChannel
	}

	baseDeckhouseConfig := map[string]interface{}{"deckhouse": deckhouseModuleConfig}

	if len(configs) == 0 {
		return baseDeckhouseConfig
	}

	var firstConfig map[string]interface{}
	_ = json.Unmarshal(configs[0], &firstConfig)

	for _, configRaw := range configs[1:] {
		var config map[string]interface{}
		_ = json.Unmarshal(configRaw, &config)

		firstConfig = mergemap.Merge(firstConfig, config)
	}

	firstConfig = mergemap.Merge(firstConfig, m.DeckhouseConfig.ConfigOverrides)
	firstConfig = mergemap.Merge(firstConfig, baseDeckhouseConfig)

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
	nodeGroup.Spec.CloudInstances["classReference"] = ClassReference{Kind: string(doc["kind"]), Name: metadata.Name}

	return json.Marshal(nodeGroup)
}

func (m *MetaConfig) MarshalConfig(_ bool) ([]byte, error) {
	return json.Marshal(m)
}

func (m *MetaConfig) MarshalClusterConfigYAML() ([]byte, error) {
	return yaml.Marshal(m.ClusterConfig)
}

func (m *MetaConfig) MarshalProviderClusterConfigYAML() ([]byte, error) {
	return yaml.Marshal(m.ProviderClusterConfig)
}

func (m *MetaConfig) MarshalConfigForKubeadmTemplates(nodeIP string) map[string]interface{} {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t interface{}
		_ = json.Unmarshal(value, &t)
		data[key] = t
	}

	return map[string]interface{}{
		"nodeIP":               nodeIP,
		"extraArgs":            make(map[string]interface{}),
		"clusterConfiguration": data,
	}
}

func (m *MetaConfig) prepareNodeGroup() map[string]interface{} {
	var data map[string]interface{}

	_ = json.Unmarshal(m.InitClusterConfig["masterNodeGroup"], &data)

	return map[string]interface{}{
		"name":     "master",
		"nodeType": "Cloud",
		"cloudInstances": map[string]interface{}{
			"classReference": map[string]string{
				"name": "master",
				"kind": m.providerNameCamelCase + "InstanceClass",
			},
		},
		"maxPerZone": data["maxPerZone"],
		"minPerZone": data["minPerZone"],
		"zones":      data["zones"],
	}
}

func (m *MetaConfig) MarshalConfigForBashibleBundleTemplate(bundle, nodeIP string) map[string]interface{} {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t interface{}
		_ = json.Unmarshal(value, &t)
		data[key] = t
	}

	ip, ipnet, _ := net.ParseCIDR(data["serviceSubnetCIDR"].(string))

	clusterDNS := ""
	counter := 0
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		// The .10 address for /24 network is dns address
		if counter == 10 {
			clusterDNS = ip.String()
			break
		}
		counter++
	}

	return map[string]interface{}{
		"runType":           "ClusterBootstrap",
		"bundle":            bundle,
		"kubernetesVersion": data["kubernetesVersion"],
		"nodeGroup":         m.prepareNodeGroup(),
		"clusterBootstrap": map[string]interface{}{
			"clusterDomain":     data["clusterDomain"],
			"nodeIP":            nodeIP,
			"clusterDNSAddress": clusterDNS,
		},
	}
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
