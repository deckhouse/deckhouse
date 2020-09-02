package config

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/peterbourgon/mergemap"
	"sigs.k8s.io/yaml"

	"flant/deckhouse-candi/pkg/log"
)

type MetaConfig struct {
	ClusterType          string `json:"-"`
	Layout               string `json:"-"`
	ProviderName         string `json:"-"`
	OriginalProviderName string `json:"-"`
	ClusterPrefix        string `json:"-"`

	DeckhouseConfig      DeckhouseClusterConfig `json:"-"`
	MasterNodeGroupSpec  MasterNodeGroupSpec    `json:"-"`
	StaticNodeGroupSpecs []StaticNodeGroupSpec  `json:"-"`

	ClusterConfig     map[string]json.RawMessage `json:"clusterConfiguration"`
	InitClusterConfig map[string]json.RawMessage `json:"-"`

	ProviderClusterConfig map[string]json.RawMessage `json:"providerClusterConfiguration"`

	UUID []byte `json:"clusterUUID,omitempty"`
}

// Prepare extracts all necessary information from raw json messages to the root structure
func (m *MetaConfig) Prepare() *MetaConfig {
	_ = json.Unmarshal(m.ClusterConfig["clusterType"], &m.ClusterType)
	_ = json.Unmarshal(m.InitClusterConfig["deckhouse"], &m.DeckhouseConfig)

	if m.ClusterType != CloudClusterType {
		return m
	}

	_ = json.Unmarshal(m.ProviderClusterConfig["layout"], &m.Layout)
	m.Layout = strcase.ToKebab(m.Layout)

	var cloud ClusterConfigCloudSpec
	_ = json.Unmarshal(m.ClusterConfig["cloud"], &cloud)
	m.ProviderName = strings.ToLower(cloud.Provider)
	m.OriginalProviderName = cloud.Provider
	m.ClusterPrefix = cloud.Prefix

	_ = json.Unmarshal(m.ProviderClusterConfig["masterNodeGroup"], &m.MasterNodeGroupSpec)
	m.StaticNodeGroupSpecs = []StaticNodeGroupSpec{}
	nodeGroups, ok := m.ProviderClusterConfig["nodeGroups"]
	if ok {
		_ = json.Unmarshal(nodeGroups, &m.StaticNodeGroupSpecs)
	}
	return m
}

// MergeDeckhouseConfig returns deckhouse config merged from different sources
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
		return mergemap.Merge(baseDeckhouseConfig, m.DeckhouseConfig.ConfigOverrides)
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

func (m *MetaConfig) GetStaticNodeGroups() []StaticNodeGroupSpec {
	return m.StaticNodeGroupSpecs
}

func (m *MetaConfig) FindStaticNodeGroup(nodeGroupName string) []byte {
	for index, ng := range m.StaticNodeGroupSpecs {
		if ng.Name == nodeGroupName {
			var staticNodeGroups []json.RawMessage
			err := json.Unmarshal(m.ProviderClusterConfig["nodeGroups"], &staticNodeGroups)
			if err != nil {
				log.ErrorLn(err)
				return nil
			}
			return staticNodeGroups[index]
		}
	}
	return nil
}

// MasterNodeGroupManifest prepares NodeGroup custom resource for master nodes
func (m *MetaConfig) MasterNodeGroupManifest() map[string]interface{} {
	nodeType := "Hybrid"
	if m.ClusterType == StaticClusterType {
		nodeType = "Static"
	}

	return map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": "master",
		},
		"spec": map[string]interface{}{
			"nodeType": nodeType,
			"disruptions": map[string]interface{}{
				"approvalMode": "Manual",
			},
			"nodeTemplate": map[string]interface{}{
				"labels": map[string]interface{}{
					"node-role.kubernetes.io/master": "",
				},
				"taints": []map[string]interface{}{
					{
						"key":    "node-role.kubernetes.io/master",
						"effect": "NoSchedule",
					},
				},
			},
		},
	}
}

// NodeGroupManifest prepares NodeGroup custom resource for static nodes, which were ordered by Terraform
func (m *MetaConfig) NodeGroupManifest(staticNodeGroup StaticNodeGroupSpec) map[string]interface{} {
	if staticNodeGroup.NodeTemplate == nil {
		staticNodeGroup.NodeTemplate = make(map[string]interface{})
	}
	return map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": staticNodeGroup.Name,
		},
		"spec": map[string]interface{}{
			"nodeType": "Hybrid",
			"disruptions": map[string]interface{}{
				"approvalMode": "Manual",
			},
			"nodeTemplate": staticNodeGroup.NodeTemplate,
		},
	}
}

func (m *MetaConfig) MarshalConfig() []byte {
	data, _ := json.Marshal(m)
	return data
}

func (m *MetaConfig) ClusterConfigYAML() ([]byte, error) {
	return yaml.Marshal(m.ClusterConfig)
}

func (m *MetaConfig) ProviderClusterConfigYAML() ([]byte, error) {
	return yaml.Marshal(m.ProviderClusterConfig)
}

func (m *MetaConfig) ConfigForKubeadmTemplates(nodeIP string) map[string]interface{} {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t interface{}
		_ = json.Unmarshal(value, &t)
		data[key] = t
	}

	result := map[string]interface{}{
		"extraArgs":            make(map[string]interface{}),
		"clusterConfiguration": data,
	}

	if nodeIP != "" {
		result["nodeIP"] = nodeIP
	}
	return result
}

func (m *MetaConfig) ConfigForBashibleBundleTemplate(bundle, nodeIP string) map[string]interface{} {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t interface{}
		_ = json.Unmarshal(value, &t)
		data[key] = t
	}

	clusterBootstrap := map[string]interface{}{
		"clusterDomain":     data["clusterDomain"],
		"clusterDNSAddress": getDNSAddress(data["serviceSubnetCIDR"].(string)),
	}

	if nodeIP != "" {
		clusterBootstrap["nodeIP"] = nodeIP
	}

	return map[string]interface{}{
		"runType":           "ClusterBootstrap",
		"bundle":            bundle,
		"kubernetesVersion": data["kubernetesVersion"],
		"nodeGroup": map[string]interface{}{
			"name":     "master",
			"nodeType": m.ClusterType,
			"cloudInstances": map[string]interface{}{
				"classReference": map[string]string{
					"name": "master",
				},
			},
		},
		"clusterBootstrap": clusterBootstrap,
	}
}

// NodeGroupConfig returns values for terraform to order master node or static node
func (m *MetaConfig) NodeGroupConfig(nodeGroupName string, nodeIndex int, cloudConfig string) []byte {
	result := map[string]interface{}{
		"clusterConfiguration":         m.ClusterConfig,
		"providerClusterConfiguration": m.ProviderClusterConfig,
		"nodeIndex":                    nodeIndex,
		"cloudConfig":                  cloudConfig,
	}

	if nodeGroupName != "master" {
		result["nodeGroupName"] = nodeGroupName
	}

	if len(m.UUID) > 0 {
		result["clusterUUID"] = m.UUID
	}

	data, _ := json.Marshal(result)
	return data
}

func (m *MetaConfig) CachePath() string {
	return fmt.Sprintf("%s-%s-terraform-state-cache", m.ClusterPrefix, m.ProviderName)
}

func (m *MetaConfig) DeepCopy() *MetaConfig {
	out := MetaConfig{}

	if m.ClusterConfig != nil {
		config := make(map[string]json.RawMessage, len(m.ClusterConfig))
		for k, v := range m.ClusterConfig {
			config[k] = v
		}
		out.ClusterConfig = config
	}

	if m.InitClusterConfig != nil {
		config := make(map[string]json.RawMessage, len(m.InitClusterConfig))
		for k, v := range m.InitClusterConfig {
			config[k] = v
		}
		out.InitClusterConfig = config
	}

	if m.ProviderClusterConfig != nil {
		config := make(map[string]json.RawMessage, len(m.ProviderClusterConfig))
		for k, v := range m.ProviderClusterConfig {
			config[k] = v
		}
		out.ProviderClusterConfig = config
	}

	return m
}

func getDNSAddress(serviceCIDR string) string {
	ip, ipnet, err := net.ParseCIDR(serviceCIDR)
	if err != nil {
		panic("serviceSubnetCIDR is not valid CIDR (should be validated with openapi scheme)")
	}

	inc := func(ip net.IP) {
		for j := len(ip) - 1; j >= 0; j-- {
			ip[j]++
			if ip[j] > 0 {
				break
			}
		}
	}

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

	return clusterDNS
}
