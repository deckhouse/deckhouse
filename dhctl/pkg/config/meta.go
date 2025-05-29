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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/iancoleman/strcase"
	"github.com/vmware/go-vcloud-director/v3/govcd"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
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

type RegistryData struct {
	Address   string `json:"address"`
	Path      string `json:"path"`
	Scheme    string `json:"scheme"`
	CA        string `json:"ca"`
	DockerCfg string `json:"dockerCfg"`
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

		parts := strings.SplitN(m.DeckhouseConfig.ImagesRepo, "/", 2)
		m.Registry.Address = parts[0]
		if len(parts) == 2 {
			m.Registry.Path = fmt.Sprintf("/%s", parts[1])
		}

		if err := validateRegistryDockerCfg(m.DeckhouseConfig.RegistryDockerCfg, m.Registry.Address); err != nil {
			return nil, err
		}
		m.Registry.DockerCfg = m.DeckhouseConfig.RegistryDockerCfg
		m.Registry.Scheme = strings.ToLower(m.DeckhouseConfig.RegistryScheme)
		m.Registry.CA = m.DeckhouseConfig.RegistryCA

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

	apiVersion := ""
	apiVersionFunc, ok := providerApiVersionsDiscoverer[cloud.Provider]

	if ok {
		var err error

		apiVersion, err = apiVersionFunc(m)
		if err != nil {
			return nil, fmt.Errorf("unable to discover provider API version: %v", err)
		}
	}

	if cloud.Provider == "Yandex" {
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

	if cloud.Provider == "VCD" {
		// Set default version for terraform-provider-vcd to 3.10.0 if legacyMode is true
		// This is a temporary solution to avoid breaking changes in the VCD API

		version, err := semver.NewVersion(apiVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse VCD API version '%s': %v", apiVersion, err)
		}

		versionConstraint, err := semver.NewConstraint("<37.2")
		if err != nil {
			return nil, fmt.Errorf("failed to parse version constraint '%s': %v", versionConstraint, err)
		}

		infrastructureModulesDir := getInfrastructureModulesDir("vcd")

		versionsFilePath := filepath.Join(infrastructureModulesDir, "versions.tf")

		err = os.Remove(versionsFilePath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove versions.tf: %v", err)
		}

		if versionConstraint.Check(version) {
			err := os.Symlink(filepath.Join(infrastructureModulesDir, "versions-legacy.tf"), versionsFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create symlink to versions-legacy.tf: %v", err)
			}
		} else {
			err := os.Symlink(filepath.Join(infrastructureModulesDir, "versions-new.tf"), versionsFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to create symlink to versions-new.tf: %v", err)
			}
		}
	}

	m.TerraNodeGroupSpecs = []TerraNodeGroupSpec{}
	nodeGroups, ok := m.ProviderClusterConfig["nodeGroups"]
	if ok {
		if err := json.Unmarshal(nodeGroups, &m.TerraNodeGroupSpecs); err != nil {
			return nil, fmt.Errorf("unable to unmarshal static nodes from provider cluster configuration: %v", err)
		}
	}

	m.Registry.DockerCfg = m.DeckhouseConfig.RegistryDockerCfg
	m.Registry.Scheme = strings.ToLower(m.DeckhouseConfig.RegistryScheme)
	m.Registry.CA = m.DeckhouseConfig.RegistryCA

	parts := strings.SplitN(m.DeckhouseConfig.ImagesRepo, "/", 2)
	m.Registry.Address = parts[0]
	if len(parts) == 2 {
		m.Registry.Path = fmt.Sprintf("/%s", parts[1])
	}

	return m, nil
}

func validateRegistryDockerCfg(cfg string, repo string) error {
	if cfg == "" {
		return fmt.Errorf("can't be empty")
	}

	regcrd, err := base64.StdEncoding.DecodeString(cfg)
	if err != nil {
		return fmt.Errorf("unable to decode registryDockerCfg: %w", err)
	}

	var creds struct {
		Auths map[string]interface{} `json:"auths"`
	}

	if err = json.Unmarshal(regcrd, &creds); err != nil {
		return fmt.Errorf("unable to unmarshal docker credentials: %w", err)
	}

	// The regexp match string with this pattern:
	// ^([a-z]|\d)+ - string starts with a [a-z] letter or a number
	// (\.?|\-?) - next symbol might be '.' or '-' and repeated zero or one times
	// (([a-z]|\d)+(\.|\-|))* - middle part of string might have [a-z] letters, numbers, '.' or ':',
	// and moreover '.' or ':' symbols can't be doubled or goes next to each other
	// ([a-z]|\d+|([a-z]|\d)\:\d+)$ - string might be ended by [a-z] letter or number (if we have single host) or
	// [a-z] letter or number with ':' symbol, and moreover there might be only numbers after ':' symbol
	regx, err := regexp.Compile(`^([a-z]|\d)+(\.?|\-?)(([a-z]|\d)+(\.|\-|))*([a-z]|\d+|([a-z]|\d)\:\d+)$`)
	if err != nil {
		return fmt.Errorf("unable to compile regexp by pattern: %w", err)
	}

	for k := range creds.Auths {
		if !regx.MatchString(k) {
			return fmt.Errorf("invalid registryDockerCfg. Your auths host \"%s\" should be similar to \"your.private.registry.example.com\"", k)
		}
	}

	for k := range creds.Auths {
		if k == repo {
			return nil
		}
	}
	return fmt.Errorf("incorrect registryDockerCfg. It must contain auths host {\"auths\": { \"%s\": {}}}", repo)
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

	registryData, err := m.ParseRegistryData()
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

	registryData, err := m.ParseRegistryData()
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

func (m *MetaConfig) ParseRegistryData() (map[string]interface{}, error) {
	log.DebugF("registry data: %v\n", m.Registry)

	ret := m.Registry.ConvertToMap()

	if m.Registry.DockerCfg != "" {
		auth, err := m.Registry.Auth()
		if err != nil {
			return nil, err
		}

		ret["auth"] = auth
	}

	return ret, nil
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

func (r *RegistryData) ConvertToMap() map[string]interface{} {
	return map[string]interface{}{
		"address":   r.Address,
		"path":      r.Path,
		"scheme":    r.Scheme,
		"ca":        r.CA,
		"dockerCfg": r.DockerCfg,
	}
}

func (r *RegistryData) Auth() (string, error) {
	type dockerCfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}

	var (
		registryAuth string
		dc           dockerCfg
	)

	bytes, err := base64.StdEncoding.DecodeString(r.DockerCfg)
	if err != nil {
		return "", fmt.Errorf("cannot base64 decode docker cfg: %v", err)
	}

	log.DebugF("parse registry data: dockerCfg after base64 decode = %s\n", bytes)
	err = json.Unmarshal(bytes, &dc)
	if err != nil {
		return "", fmt.Errorf("cannot unmarshal docker cfg: %v", err)
	}

	if registry, ok := dc.Auths[r.Address]; ok {
		switch {
		case registry.Auth != "":
			registryAuth = registry.Auth
		case registry.Username != "" && registry.Password != "":
			auth := fmt.Sprintf("%s:%s", registry.Username, registry.Password)
			registryAuth = base64.StdEncoding.EncodeToString([]byte(auth))
		default:
			log.DebugF("auth or username with password not found in dockerCfg %s for %s. Use empty string", bytes, r.Address)
		}
	}

	return registryAuth, nil
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

type providerApiVersionsDiscoverMap map[string]func(*MetaConfig) (string, error)

var providerApiVersionsDiscoverer = providerApiVersionsDiscoverMap{
	"VCD": (*MetaConfig).discoverVCDApiVersion,
}

type VCDProviderConfig struct {
	Server   string `json:"server"`
	Insecure bool   `json:"insecure,omitempty"`
}

func (m *MetaConfig) discoverVCDApiVersion() (string, error) {
	if m.ClusterType != CloudClusterType || len(m.ProviderClusterConfig) == 0 {
		return "", fmt.Errorf("current cluster type is not a cloud type")
	}

	var cloud ClusterConfigCloudSpec
	if err := json.Unmarshal(m.ClusterConfig["cloud"], &cloud); err != nil {
		return "", fmt.Errorf("unable to unmarshal cloud section from provider cluster configuration: %v", err)
	}

	if cloud.Provider != "VCD" {
		return "", fmt.Errorf("current provider type is not VCD")
	}

	var providerConfiguration VCDProviderConfig
	if err := json.Unmarshal(m.ProviderClusterConfig["provider"], &providerConfiguration); err != nil {
		return "", fmt.Errorf("unable to unmarshal provider configuration: %v", err)
	}

	vcdUrl, err := url.ParseRequestURI(fmt.Sprintf("%s/api", providerConfiguration.Server))
	if err != nil {
		return "", nil
	}
	insecure := providerConfiguration.Insecure

	vcdClient := govcd.NewVCDClient(
		*vcdUrl,
		insecure,
	)

	vcdClient.Client.APIVCDMaxVersionIs("")

	return vcdClient.Client.MaxSupportedVersion()
}
