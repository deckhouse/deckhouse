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
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/iancoleman/strcase"
	"github.com/vmware/go-vcloud-director/v3/govcd"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type MetaConfig struct {
	ClusterType          string                 `json:"-"`
	Layout               string                 `json:"-"`
	ProviderName         string                 `json:"-"`
	OriginalProviderName string                 `json:"-"`
	ClusterPrefix        string                 `json:"-"`
	ClusterDNSAddress    string                 `json:"-"`
	DeckhouseConfig      DeckhouseClusterConfig `json:"-"`
	MasterNodeGroupSpec  MasterNodeGroupSpec    `json:"-"`
	TerraNodeGroupSpecs  []TerraNodeGroupSpec   `json:"-"`

	ClusterConfig     map[string]json.RawMessage `json:"clusterConfiguration"`
	InitClusterConfig map[string]json.RawMessage `json:"-"`
	ModuleConfigs     []*ModuleConfig            `json:"-"`

	ProviderClusterConfig map[string]json.RawMessage `json:"providerClusterConfiguration,omitempty"`
	StaticClusterConfig   map[string]json.RawMessage `json:"staticClusterConfiguration,omitempty"`

	VersionMap                map[string]interface{} `json:"-"`
	Images                    imagesDigests          `json:"-"`
	Registry                  RegistryData           `json:"-"`
	UUID                      string                 `json:"clusterUUID,omitempty"`
	InstallerVersion          string                 `json:"-"`
	ResourcesYAML             string                 `json:"-"`
	ResourceManagementTimeout string                 `json:"resourceManagementTimeout,omitempty"`
}

type imagesDigests map[string]map[string]interface{}

type providerApiDiscoverMap map[string]func(*MetaConfig) (any, error)

var providerApiDiscoverer = providerApiDiscoverMap{
	ProviderVCD: (*MetaConfig).discoverVCDApi,
}

// Prepare extracts all necessary information from raw json messages to the root structure
func (m *MetaConfig) Prepare() (*MetaConfig, error) {
	if len(m.ClusterConfig) > 0 {
		if err := json.Unmarshal(m.ClusterConfig["clusterType"], &m.ClusterType); err != nil {
			return nil, fmt.Errorf("unable to parse cluster type from cluster configuration: %v", err)
		}

		var serviceSubnet string
		if err := json.Unmarshal(m.ClusterConfig["serviceSubnetCIDR"], &serviceSubnet); err != nil {
			return nil, fmt.Errorf("unable to unmarshal service subnet CIDR from cluster configuration: %v", err)
		}
		m.ClusterDNSAddress = getDNSAddress(serviceSubnet)
	}

	if len(m.InitClusterConfig) > 0 {
		if err := json.Unmarshal(m.InitClusterConfig["deckhouse"], &m.DeckhouseConfig); err != nil {
			return nil, fmt.Errorf("unable to unmarshal deckhouse configuration: %v", err)
		}

		imagesRepo := strings.TrimSpace(m.DeckhouseConfig.ImagesRepo)
		m.DeckhouseConfig.ImagesRepo = strings.TrimRight(imagesRepo, "/")
		m.Registry.Process(m.DeckhouseConfig)
	}

	if m.ClusterType != CloudClusterType || len(m.ProviderClusterConfig) == 0 {
		return m, nil
	}

	if err := json.Unmarshal(m.ProviderClusterConfig["layout"], &m.Layout); err != nil {
		return nil, fmt.Errorf("unable to unmarshal layout from cluster configuration: %v", err)
	}
	m.Layout = strcase.ToKebab(m.Layout)

	var cloud ClusterConfigCloudSpec
	if err := json.Unmarshal(m.ClusterConfig["cloud"], &cloud); err != nil {
		return nil, fmt.Errorf("unable to unmarshal cloud section from provider cluster configuration: %v", err)
	}

	m.ProviderName = strings.ToLower(cloud.Provider)
	m.OriginalProviderName = cloud.Provider
	m.ClusterPrefix = cloud.Prefix

	if err := json.Unmarshal(m.ProviderClusterConfig["masterNodeGroup"], &m.MasterNodeGroupSpec); err != nil {
		return nil, fmt.Errorf("unable to unmarshal master node group from provider cluster configuration: %v", err)
	}

	var providerInfo interface{}
	providerAPIInfoFunc, ok := providerApiDiscoverer[cloud.Provider]

	if ok {
		var err error

		providerInfo, err = providerAPIInfoFunc(m)
		if err != nil {
			return nil, fmt.Errorf("unable to discover provider info: %v", err)
		}
	}

	if cloud.Provider == ProviderYandex {
		if err := ValidateClusterConfigurationPrefix(cloud.Prefix, cloud.Provider); err != nil {
			return nil, err
		}

		var masterNodeGroup YandexMasterNodeGroupSpec
		if err := json.Unmarshal(m.ProviderClusterConfig["masterNodeGroup"], &masterNodeGroup); err != nil {
			return nil, fmt.Errorf("unable to unmarshal master node group from provider cluster configuration: %v", err)
		}

		if masterNodeGroup.Replicas > 0 &&
			len(masterNodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
			masterNodeGroup.Replicas > len(masterNodeGroup.InstanceClass.ExternalIPAddresses) {
			return nil, fmt.Errorf("number of masterNodeGroup.replicas should be equal to the length of masterNodeGroup.instanceClass.externalIPAddresses")
		}

		nodeGroups, ok := m.ProviderClusterConfig["nodeGroups"]
		if ok {
			var yandexNodeGroups []YandexNodeGroupSpec
			if err := json.Unmarshal(nodeGroups, &yandexNodeGroups); err != nil {
				return nil, fmt.Errorf("unable to unmarshal node groups from provider cluster configuration: %v", err)
			}

			for _, nodeGroup := range yandexNodeGroups {
				if nodeGroup.Replicas > 0 &&
					len(nodeGroup.InstanceClass.ExternalIPAddresses) > 0 &&
					nodeGroup.Replicas > len(nodeGroup.InstanceClass.ExternalIPAddresses) {
					return nil, fmt.Errorf(`number of nodeGroups["%s"].replicas should be equal to the length of nodeGroups["%s"].instanceClass.externalIPAddresses`, nodeGroup.Name, nodeGroup.Name)
				}
			}
		}
	}

	if cloud.Provider == ProviderVCD {
		// Set default version for terraform-provider-vcd to 3.10.0 if VCD API version is less than 37.2
		// This is a temporary solution to avoid breaking changes in the VCD API

		VCDProviderInfo, ok := providerInfo.(*VCDProviderInfo)
		if !ok {
			return nil, fmt.Errorf("failed to get VCD provider info")
		}

		version, err := semver.NewVersion(VCDProviderInfo.ApiVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse VCD API version '%s': %v", VCDProviderInfo.ApiVersion, err)
		}

		log.DebugF("VCD API version '%s'\n", VCDProviderInfo.ApiVersion)

		const versionConstraintStr = "<37.2"

		versionConstraint, err := semver.NewConstraint(versionConstraintStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version constraint '%s': %v", versionConstraint, err)
		}

		infrastructureModulesDir := infrastructure.GetInfrastructureModulesDir("vcd")

		versionsFilePath := filepath.Join(infrastructureModulesDir, "versions.tf")

		log.DebugF("Infrastructure version file for VCD %s\n", versionsFilePath)

		err = os.Remove(versionsFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to remove versions.tf: %v", err)
			}

			log.DebugF("Infrastructure version file %s not found. Continue\n", versionsFilePath)
		} else {
			log.DebugF("Infrastructure version file %s was found and deleted\n", versionsFilePath)
		}

		if versionConstraint.Check(version) {
			log.DebugF("Use legacy VCD version %s (%s). Use legacy mode as true\n", version, versionConstraintStr)
			if _, ok := m.ProviderClusterConfig["legacyMode"]; !ok {
				legacyMode, err := json.Marshal(true)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal legacyMode: %v", err)
				}

				m.ProviderClusterConfig["legacyMode"] = legacyMode
			}

			err = os.Symlink(filepath.Join(infrastructureModulesDir, "versions-legacy.tf"), versionsFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create symlink to versions-legacy.tf: %v", err)
			}

			log.DebugLn("Symlink to legacy version file was created\n")
		} else {
			log.DebugF("Use latest VCD version %s (%s)e\n", version, versionConstraintStr)

			err := os.Symlink(filepath.Join(infrastructureModulesDir, "versions-new.tf"), versionsFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create symlink to versions-new.tf: %v", err)
			}

			log.DebugLn("Symlink to latest version file was created\n")
		}
	}

	m.TerraNodeGroupSpecs = []TerraNodeGroupSpec{}
	nodeGroups, ok := m.ProviderClusterConfig["nodeGroups"]
	if ok {
		if err := json.Unmarshal(nodeGroups, &m.TerraNodeGroupSpecs); err != nil {
			return nil, fmt.Errorf("unable to unmarshal static nodes from provider cluster configuration: %v", err)
		}
	}

	return m, nil
}

func (m *MetaConfig) GetTerraNodeGroups() []TerraNodeGroupSpec {
	return m.TerraNodeGroupSpecs
}

func (m *MetaConfig) FindTerraNodeGroup(nodeGroupName string) []byte {
	for index, ng := range m.TerraNodeGroupSpecs {
		if ng.Name == nodeGroupName {
			var terraNodeGroups []json.RawMessage
			err := json.Unmarshal(m.ProviderClusterConfig["nodeGroups"], &terraNodeGroups)
			if err != nil {
				log.ErrorLn(err)
				return nil
			}
			return terraNodeGroups[index]
		}
	}
	return nil
}

func (m *MetaConfig) IsStatic() bool {
	return m.ClusterType == "Static"
}

func (m *MetaConfig) ExtractMasterNodeGroupStaticSettings() map[string]interface{} {
	static := make(map[string]interface{})

	if len(m.StaticClusterConfig) == 0 {
		return static
	}

	var internalNetworkCIDRs []string
	if data, ok := m.StaticClusterConfig["internalNetworkCIDRs"]; ok {
		err := json.Unmarshal(data, &internalNetworkCIDRs)
		if err != nil {
			log.DebugF("unmarshalling internalNetworkCIDRs: %v\n", err)
			return static
		}
	}

	static["internalNetworkCIDRs"] = internalNetworkCIDRs
	return static
}

// NodeGroupManifest prepares NodeGroup custom resource for static nodes, which were ordered by infrastructure utility
func (m *MetaConfig) NodeGroupManifest(terraNodeGroup TerraNodeGroupSpec) map[string]interface{} {
	if terraNodeGroup.NodeTemplate == nil {
		terraNodeGroup.NodeTemplate = make(map[string]interface{})
	}
	return map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": terraNodeGroup.Name,
		},
		"spec": map[string]interface{}{
			"nodeType": "CloudPermanent",
			"disruptions": map[string]interface{}{
				"approvalMode": "Manual",
			},
			"nodeTemplate": terraNodeGroup.NodeTemplate,
		},
	}
}

func (m *MetaConfig) MarshalFullConfig() []byte {
	data, _ := json.Marshal(m)
	return data
}

func (m *MetaConfig) MarshalConfig() []byte {
	newM := m.DeepCopy()
	newM.StaticClusterConfig = nil
	data, _ := json.Marshal(newM)
	return data
}

func (m *MetaConfig) ClusterConfigYAML() ([]byte, error) {
	if m.ClusterConfig == nil {
		return []byte{}, nil
	}
	return yaml.Marshal(m.ClusterConfig)
}

func (m *MetaConfig) ProviderClusterConfigYAML() ([]byte, error) {
	if m.ProviderClusterConfig == nil {
		return []byte{}, nil
	}
	return yaml.Marshal(m.ProviderClusterConfig)
}

func (m *MetaConfig) StaticClusterConfigYAML() ([]byte, error) {
	if m.StaticClusterConfig == nil {
		return []byte{}, nil
	}
	return yaml.Marshal(m.StaticClusterConfig)
}

func (m *MetaConfig) ConfigForKubeadmTemplates(nodeIP string) (map[string]interface{}, error) {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t interface{}
		err := json.Unmarshal(value, &t)
		if err != nil {
			return nil, fmt.Errorf("cluster config unmarshal: %v", err)
		}
		data[key] = t
	}

	if data["kubernetesVersion"] == "Automatic" {
		data["kubernetesVersion"] = DefaultKubernetesVersion
	}

	result := make(map[string]interface{})
	for key, value := range m.VersionMap {
		result[key] = value
	}

	result["runType"] = "ClusterBootstrap"
	result["extraArgs"] = make(map[string]interface{})
	result["clusterConfiguration"] = data
	// bashible will use this as a placeholder on envsubst call, address will be discovered in one of bashible steps
	result["nodeIP"] = "$MY_IP"

	if nodeIP != "" {
		result["nodeIP"] = nodeIP
	}

	registryData, err := m.Registry.KubeadmTemplatesCtx()
	if err != nil {
		return nil, err
	}

	result["registry"] = registryData

	images := m.Images

	result["images"] = images.ConvertToMap()
	return result, nil
}

func (m *MetaConfig) ConfigForBashibleBundleTemplate(nodeIP string) (map[string]interface{}, error) {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t interface{}
		err := json.Unmarshal(value, &t)
		if err != nil {
			return nil, fmt.Errorf("cluster config unmarshal: %v", err)
		}
		data[key] = t
	}

	if data["kubernetesVersion"] == "Automatic" {
		data["kubernetesVersion"] = DefaultKubernetesVersion
	}

	clusterBootstrap := map[string]interface{}{
		"clusterDomain":     data["clusterDomain"],
		"clusterDNSAddress": m.ClusterDNSAddress,
	}

	if nodeIP != "" {
		clusterBootstrap["cloud"] = map[string]interface{}{"nodeIP": nodeIP}
	}

	nodeGroup := map[string]interface{}{
		"name":     "master",
		"nodeType": "CloudPermanent",
		"cloudInstances": map[string]interface{}{
			"classReference": map[string]string{
				"name": "master",
			},
		},
	}

	if m.ClusterType == StaticClusterType {
		nodeGroup["nodeType"] = "Static"
		nodeGroup["static"] = m.ExtractMasterNodeGroupStaticSettings()
	}

	registryData, err := m.Registry.BashibleBundleTemplateCtx()
	if err != nil {
		return nil, err
	}

	configForBashibleBundleTemplate := make(map[string]interface{})
	for key, value := range m.VersionMap {
		configForBashibleBundleTemplate[key] = value
	}

	configForBashibleBundleTemplate["runType"] = "ClusterBootstrap"

	if m.ClusterType == CloudClusterType {
		configForBashibleBundleTemplate["provider"] = m.ProviderName
	}

	configForBashibleBundleTemplate["cri"] = data["defaultCRI"]
	configForBashibleBundleTemplate["kubernetesVersion"] = data["kubernetesVersion"]
	configForBashibleBundleTemplate["nodeGroup"] = nodeGroup
	configForBashibleBundleTemplate["clusterBootstrap"] = clusterBootstrap
	configForBashibleBundleTemplate["proxy"] = make(map[string]interface{})
	if data["proxy"] != nil {
		proxyData, err := m.EnrichProxyData()
		if err != nil {
			return nil, err
		}
		if proxyData != nil {
			configForBashibleBundleTemplate["proxy"] = proxyData
		}
	}
	configForBashibleBundleTemplate["registry"] = registryData

	images := m.Images
	configForBashibleBundleTemplate["images"] = images.ConvertToMap()

	configForBashibleBundleTemplate["packagesProxy"] = map[string]interface{}{"addresses": []string{"127.0.0.1:5444"}}
	return configForBashibleBundleTemplate, nil
}

// NodeGroupConfig returns values for infrastructure utility to order master node or static node
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

	if len(m.ResourceManagementTimeout) > 0 {
		result["resourceManagementTimeout"] = m.ResourceManagementTimeout
	}

	data, _ := json.Marshal(result)
	return data
}

func (m *MetaConfig) CachePath() string {
	return fmt.Sprintf("%s-%s-terraform-state-cache", m.ClusterPrefix, m.ProviderName)
}

func (m *MetaConfig) DeepCopy() *MetaConfig {
	out := &MetaConfig{}

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

	if m.StaticClusterConfig != nil {
		config := make(map[string]json.RawMessage, len(m.StaticClusterConfig))
		for k, v := range m.StaticClusterConfig {
			config[k] = v
		}
		out.StaticClusterConfig = config
	}

	out.Registry = m.Registry

	if m.ClusterType != "" {
		out.ClusterType = m.ClusterType
	}

	if m.ClusterPrefix != "" {
		out.ClusterPrefix = m.ClusterPrefix
	}

	if m.Layout != "" {
		out.Layout = m.Layout
	}

	if m.ProviderName != "" {
		out.ProviderName = m.ProviderName
	}

	if m.OriginalProviderName != "" {
		out.OriginalProviderName = m.OriginalProviderName
	}

	if m.UUID != "" {
		out.UUID = m.UUID
	}

	if m.ResourceManagementTimeout != "" {
		out.ResourceManagementTimeout = m.ResourceManagementTimeout
	}

	return out
}

func (m *MetaConfig) LoadVersionMap(filename string) error {
	versionMap := make(map[string]interface{})

	versionMapFile, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("%s file load: %v", filename, err)
	}

	err = yaml.Unmarshal(versionMapFile, &versionMap)
	if err != nil {
		return fmt.Errorf("%s file unmarshal: %v", filename, err)
	}

	m.VersionMap = versionMap

	return nil
}

func (m *MetaConfig) EnrichProxyData() (map[string]interface{}, error) {
	type proxy struct {
		HttpProxy  string   `json:"httpProxy" yaml:"httpProxy"`
		HttpsProxy string   `json:"httpsProxy" yaml:"httpsProxy"`
		NoProxy    []string `json:"noProxy" yaml:"noProxy"`
	}

	p := &proxy{}
	cp, ok := m.ClusterConfig["proxy"]
	if !ok {
		return nil, nil
	}

	err := json.Unmarshal(cp, &p)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal proxy cfg: %v", err)
	}

	var (
		clusterDomain     string
		podSubnetCIDR     string
		serviceSubnetCIDR string
	)
	err = json.Unmarshal(m.ClusterConfig["clusterDomain"], &clusterDomain)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(m.ClusterConfig["podSubnetCIDR"], &podSubnetCIDR)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(m.ClusterConfig["serviceSubnetCIDR"], &serviceSubnetCIDR)
	if err != nil {
		return nil, err
	}

	p.NoProxy = append(p.NoProxy, "127.0.0.1", "169.254.169.254", clusterDomain, podSubnetCIDR, serviceSubnetCIDR)

	ret := make(map[string]interface{})
	if p.HttpProxy != "" {
		ret["httpProxy"] = p.HttpProxy
	}
	if p.HttpsProxy != "" {
		ret["httpsProxy"] = p.HttpsProxy
	}
	ret["noProxy"] = p.NoProxy

	return ret, nil
}

func (m *MetaConfig) LoadImagesDigests(filename string) error {
	var imagesDigests imagesDigests

	imagesDigestsJSONFile, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("%s file load: %v", filename, err)
	}

	err = yaml.Unmarshal(imagesDigestsJSONFile, &imagesDigests)
	if err != nil {
		return fmt.Errorf("%s file unmarshal: %v", filename, err)
	}

	m.Images = imagesDigests

	return nil
}

func (m *MetaConfig) LoadInstallerVersion() error {
	rawFile, err := os.ReadFile(app.VersionFile)
	if err != nil {
		return err
	}

	m.InstallerVersion = strings.TrimSpace(string(rawFile))

	return nil
}

func (m *MetaConfig) GetReplicasByNodeGroupName(nodeGroupName string) int {
	if nodeGroupName == global.MasterNodeGroupName {
		return m.MasterNodeGroupSpec.Replicas
	}

	for _, group := range m.GetTerraNodeGroups() {
		if group.Name == nodeGroupName {
			return group.Replicas
		}
	}

	return 0
}

func getDNSAddress(serviceCIDR string) string {
	ip, ipnet, err := net.ParseCIDR(serviceCIDR)
	if err != nil {
		log.DebugLn("serviceSubnetCIDR is not valid CIDR (should be validated with openapi scheme)")
		return ""
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

func (i *imagesDigests) ConvertToMap() map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range *i {
		res[k] = v
	}
	return res
}

func GetIndexFromNodeName(name string) (int, error) {
	index, err := strconv.Atoi(name[strings.LastIndex(name, "-")+1:])
	if err != nil {
		return 0, err
	}
	return index, nil
}

func (m *MetaConfig) discoverVCDApi() (any, error) {
	if m.ClusterType != CloudClusterType || len(m.ProviderClusterConfig) == 0 {
		return nil, fmt.Errorf("current cluster type is not a cloud type")
	}

	var cloud ClusterConfigCloudSpec
	if err := json.Unmarshal(m.ClusterConfig["cloud"], &cloud); err != nil {
		return nil, fmt.Errorf("unable to unmarshal cloud section from provider cluster configuration: %v", err)
	}

	if cloud.Provider != ProviderVCD {
		return nil, fmt.Errorf("current provider type is not VCD")
	}

	var providerConfiguration VCDProviderConfig
	if err := json.Unmarshal(m.ProviderClusterConfig["provider"], &providerConfiguration); err != nil {
		return nil, fmt.Errorf("unable to unmarshal provider configuration: %v", err)
	}

	vcdUrl, err := url.ParseRequestURI(fmt.Sprintf("%s/api", providerConfiguration.Server))
	if err != nil {
		return nil, fmt.Errorf("unable to parse VCD provider url: %v", err)
	}
	insecure := providerConfiguration.Insecure

	vcdClient := govcd.NewVCDClient(
		*vcdUrl,
		insecure,
	)

	vcdClient.Client.APIVCDMaxVersionIs("")

	apiVersion, err := vcdClient.Client.MaxSupportedVersion()
	if err != nil {
		return nil, fmt.Errorf("unable to get VCD API version: %v", err)
	}

	return &VCDProviderInfo{ApiVersion: apiVersion}, nil
}
