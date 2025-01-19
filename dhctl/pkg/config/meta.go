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
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	util_time "github.com/deckhouse/deckhouse/dhctl/pkg/util/time"
)

type MetaConfig struct {
	ClusterType          string                 `json:"-"`
	Layout               string                 `json:"-"`
	ProviderName         string                 `json:"-"`
	OriginalProviderName string                 `json:"-"`
	ClusterPrefix        string                 `json:"-"`
	ClusterDNSAddress    string                 `json:"-"`
	DeckhouseConfig      DeckhouseClusterConfig `json:"-"`
	RegistryConfig       RegistryClusterConfig  `json:"-"`
	MasterNodeGroupSpec  MasterNodeGroupSpec    `json:"-"`
	TerraNodeGroupSpecs  []TerraNodeGroupSpec   `json:"-"`

	ClusterConfig        map[string]json.RawMessage `json:"clusterConfiguration"`
	InitClusterConfig    map[string]json.RawMessage `json:"-"`
	SystemRegistryConfig SystemRegistryConfig       `json:"-"`
	ModuleConfigs        []*ModuleConfig            `json:"-"`

	ProviderClusterConfig map[string]json.RawMessage `json:"providerClusterConfiguration,omitempty"`
	StaticClusterConfig   map[string]json.RawMessage `json:"staticClusterConfiguration,omitempty"`

	VersionMap       map[string]interface{} `json:"-"`
	Images           imagesDigests          `json:"-"`
	Registry         Registry               `json:"-"`
	UUID             string                 `json:"clusterUUID,omitempty"`
	InstallerVersion string                 `json:"-"`
	ResourcesYAML    string                 `json:"-"`
}

type imagesDigests map[string]map[string]interface{}

type Registry struct {
	Data               RegistryData `json:"-"`
	ModeSpecificFields interface{}  `json:"-"`
}

type RegistryData struct {
	Address   string `json:"address"`
	Path      string `json:"path"`
	Scheme    string `json:"scheme"`
	CA        string `json:"ca"`
	DockerCfg string `json:"dockerCfg"`
}

type ProxyModeRegistryData struct {
	UpstreamRegistryData   RegistryData       `json:"-"`
	InternalRegistryAccess RegistryAccessData `json:"-"`
	TTL                    util_time.Duration `json:"-"`
}

type DetachedModeRegistryData struct {
	RegistryPath           string             `json:"-"`
	ImagesBundlePath       string             `json:"-"`
	InternalRegistryAccess RegistryAccessData `json:"-"`
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
		if err := m.prepareDataFromInitClusterConfig(); err != nil {
			return nil, err
		}
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

	m.TerraNodeGroupSpecs = []TerraNodeGroupSpec{}
	nodeGroups, ok := m.ProviderClusterConfig["nodeGroups"]
	if ok {
		if err := json.Unmarshal(nodeGroups, &m.TerraNodeGroupSpecs); err != nil {
			return nil, fmt.Errorf("unable to unmarshal static nodes from provider cluster configuration: %v", err)
		}
	}
	return m, nil
}

// PrepareAfterGlobalCacheInit Some of the information from the metaconfig is used to create a global cache.
// This function is necessary to initialize the data after creating the global cache
func (m *MetaConfig) PrepareAfterGlobalCacheInit() error {
	type DockerCfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}

	if len(m.InitClusterConfig) > 0 {
		if !m.Registry.IsDirect() {
			internalRegistryAccessData, err := getRegistryAccessData()
			if err != nil {
				return fmt.Errorf("unable to get internal registry access data: %v", err)
			}

			dockerCfg, err := json.Marshal(
				DockerCfg{
					Auths: map[string]struct {
						Auth string `json:"auth"`
					}{
						m.Registry.Data.Address: {
							Auth: base64.StdEncoding.EncodeToString(
								[]byte(fmt.Sprintf(
									"%s:%s",
									internalRegistryAccessData.UserRo.Name,
									internalRegistryAccessData.UserRo.Password,
								)),
							),
						},
					},
				},
			)
			if err != nil {
				return fmt.Errorf("cannot marshal docker cfg: %v", err)
			}

			switch m.Registry.ModeSpecificFields.(type) {
			case ProxyModeRegistryData:
				modeSpecificFields := m.Registry.ModeSpecificFields.(ProxyModeRegistryData)
				modeSpecificFields.InternalRegistryAccess = *internalRegistryAccessData
				m.Registry.ModeSpecificFields = modeSpecificFields
			case DetachedModeRegistryData:
				modeSpecificFields := m.Registry.ModeSpecificFields.(DetachedModeRegistryData)
				modeSpecificFields.InternalRegistryAccess = *internalRegistryAccessData
				m.Registry.ModeSpecificFields = modeSpecificFields
			}

			m.Registry.Data.DockerCfg = string(base64.StdEncoding.EncodeToString(dockerCfg))
			m.Registry.Data.Scheme = "https"
			m.Registry.Data.CA = (*internalRegistryAccessData).CA.Cert
		}
	}
	return nil
}

func (m *MetaConfig) prepareDataFromInitClusterConfig() error {
	// Migrate from old to new init config apiVersion
	var initCfgApiVersion string
	var dockerCfg string
	if err := json.Unmarshal(m.InitClusterConfig["apiVersion"], &initCfgApiVersion); err != nil {
		return fmt.Errorf("unable to unmarshal apiVersion for init configuration: %v", err)
	}
	if initCfgApiVersion != "deckhouse.io/v2alpha1" {
		var deckhouseCfgOld DeckhouseClusterConfigOld
		if err := json.Unmarshal(m.InitClusterConfig["deckhouse"], &deckhouseCfgOld); err != nil {
			return fmt.Errorf("unable to unmarshal deckhouse configuration: %v", err)
		} else {
			m.RegistryConfig = RegistryClusterConfig{
				Mode: RegistryModeDirect,
				DirectModeProperties: &RegistryDirectModeProperties{
					ImagesRepo: deckhouseCfgOld.ImagesRepo,
					DockerCfg:  deckhouseCfgOld.RegistryDockerCfg,
					CA:         deckhouseCfgOld.RegistryCA,
					Scheme:     deckhouseCfgOld.RegistryScheme,
				},
			}
			m.DeckhouseConfig = DeckhouseClusterConfig{
				ReleaseChannel:  deckhouseCfgOld.ReleaseChannel,
				DevBranch:       deckhouseCfgOld.DevBranch,
				Bundle:          deckhouseCfgOld.Bundle,
				LogLevel:        deckhouseCfgOld.LogLevel,
				ConfigOverrides: deckhouseCfgOld.ConfigOverrides,
			}

			registryAddress := getRegistryAddressFromImagesRepo(deckhouseCfgOld.ImagesRepo)
			if err = validateRegistryDockerCfg(deckhouseCfgOld.RegistryDockerCfg, registryAddress); err != nil {
				return err
			}

		}
	} else {
		if err := json.Unmarshal(m.InitClusterConfig["deckhouse"], &m.DeckhouseConfig); err != nil {
			return fmt.Errorf("unable to unmarshal deckhouse configuration: %v", err)
		}
		if err := json.Unmarshal(m.InitClusterConfig["registry"], &m.RegistryConfig); err != nil {
			return fmt.Errorf("unable to unmarshal registry configuration: %v", err)
		}
	}

	embeddedRegistryPath := "/system/deckhouse"
	switch m.RegistryConfig.Mode {
	case RegistryModeDirect:
		properties := m.RegistryConfig.DirectModeProperties
		if properties == nil {
			return fmt.Errorf("unable to get the properties of the direct registry mode")
		}
		address, path := getRegistryAddressAndPathFromImagesRepo(properties.ImagesRepo)
		if initCfgApiVersion != "deckhouse.io/v2alpha1" {
			dockerCfg = properties.DockerCfg
		} else {
			var err error
			dockerCfg, err = generateDockerCfgBase64(properties.Username, properties.Password, address)
			if err != nil {
				return err
			}
		}

		m.Registry = Registry{
			Data: RegistryData{
				Address:   address,
				Path:      path,
				Scheme:    strings.ToLower(properties.Scheme),
				CA:        properties.CA,
				DockerCfg: dockerCfg,
			},
		}
	case RegistryModeProxy:
		m.SystemRegistryConfig.Enable = true
		properties := m.RegistryConfig.ProxyModeProperties
		if properties == nil {
			return fmt.Errorf("unable to get the properties of the proxy registry mode")
		}
		address, path := getRegistryAddressAndPathFromImagesRepo(properties.ImagesRepo)
		if initCfgApiVersion != "deckhouse.io/v2alpha1" {
			dockerCfg = properties.DockerCfg
		} else {
			var err error
			dockerCfg, err = generateDockerCfgBase64(properties.Username, properties.Password, address)
			if err != nil {
				return err
			}
		}

		m.Registry = Registry{
			Data: RegistryData{
				Address: "embedded-registry.d8-system.svc:5001",
				Path:    embeddedRegistryPath,
				// These parameters are filled in the method `PrepareAfterGlobalCacheInit`:
				// Scheme:       "",
				// DockerCfg:    "",
				// CA:           "",
			},
			ModeSpecificFields: ProxyModeRegistryData{
				UpstreamRegistryData: RegistryData{
					Address:   address,
					Path:      path,
					Scheme:    strings.ToLower(properties.Scheme),
					CA:        properties.CA,
					DockerCfg: dockerCfg,
				},
				TTL: properties.TTL,
			},
		}
	case RegistryModeDetached:
		m.SystemRegistryConfig.Enable = true
		properties := m.RegistryConfig.DetachedModeProperties
		if properties == nil {
			return fmt.Errorf("unable to get the properties of the detached registry mode")
		}

		m.Registry = Registry{
			Data: RegistryData{
				Address: "embedded-registry.d8-system.svc:5001",
				Path:    embeddedRegistryPath,
				// These parameters are filled in the method `PrepareAfterGlobalCacheInit`:
				// Scheme:       "",
				// DockerCfg:    "",
				// CA:           "",
			},
			ModeSpecificFields: DetachedModeRegistryData{
				RegistryPath:     embeddedRegistryPath,
				ImagesBundlePath: properties.ImagesBundlePath,
			},
		}
	}
	return nil
}

// getRegistryAddressFromImagesRepo returns the registry address from the given image repository.
func getRegistryAddressFromImagesRepo(imgRepo string) string {
	return strings.SplitN(strings.TrimSpace(strings.TrimRight(imgRepo, "/")), "/", 2)[0]
}

// getRegistryAddressAndPathFromImagesRepo returns the registry address and path from the given image repository.
func getRegistryAddressAndPathFromImagesRepo(imgRepo string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(strings.TrimRight(imgRepo, "/")), "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}

// generateDockerCfgBase64 creates a base64-encoded Docker config.json with credentials for a given registry.
func generateDockerCfgBase64(username, password, registryAddress string) (string, error) {
	// Create the "auth" field by base64-encoding "username:password"
	authString := fmt.Sprintf("%s:%s", username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// Build Docker config JSON structure
	type authEntry struct {
		Auth     string `json:"auth"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	type dockerCfg struct {
		Auths map[string]authEntry `json:"auths"`
	}

	cfg := dockerCfg{
		Auths: map[string]authEntry{
			registryAddress: {
				Auth:     encodedAuth,
				Username: username,
				Password: password,
			},
		},
	}

	// Convert the config structure to JSON
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal DockerCfg JSON: %w", err)
	}

	// Encode the JSON to a base64 string
	return base64.StdEncoding.EncodeToString(jsonBytes), nil
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

func (m *MetaConfig) GetClusterDomain() (string, error) {
	var clusterDomain string
	if err := json.Unmarshal(m.ClusterConfig["clusterDomain"], &clusterDomain); err != nil {
		return clusterDomain, fmt.Errorf("unable to unmarshal clusterDomain from cluster configuration: %v", err)
	}
	return clusterDomain, nil
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

// NodeGroupManifest prepares NodeGroup custom resource for static nodes, which were ordered by Terraform
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

	registryData, err := m.Registry.ConvertToMap()
	if err != nil {
		return nil, err
	}

	result["registry"] = registryData

	images := m.Images

	result["images"] = images.ConvertToMap()
	return result, nil
}

func (m *MetaConfig) ConfigForBashibleBundleTemplate(bundle, nodeIP string) (map[string]interface{}, error) {
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

	registryData, err := m.Registry.ConvertToMap()
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

	configForBashibleBundleTemplate["bundle"] = bundle
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
	} else {
		result["systemRegistryEnable"] = m.SystemRegistryConfig.Enable
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

	out.Registry = m.Registry.DeepCopy()
	out.SystemRegistryConfig = m.SystemRegistryConfig

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

func (rData ProxyModeRegistryData) DeepCopy() ProxyModeRegistryData {
	return ProxyModeRegistryData{
		UpstreamRegistryData:   rData.UpstreamRegistryData,
		InternalRegistryAccess: rData.InternalRegistryAccess,
		TTL:                    rData.TTL,
	}
}

func (rData DetachedModeRegistryData) DeepCopy() DetachedModeRegistryData {
	return DetachedModeRegistryData{
		RegistryPath:           rData.RegistryPath,
		ImagesBundlePath:       rData.ImagesBundlePath,
		InternalRegistryAccess: rData.InternalRegistryAccess,
	}
}

func (rData *RegistryData) ConvertToMap() (map[string]interface{}, error) {
	data := map[string]interface{}{
		"address":   rData.Address,
		"path":      rData.Path,
		"scheme":    rData.Scheme,
		"ca":        rData.CA,
		"dockerCfg": rData.DockerCfg,
	}

	if rData.DockerCfg != "" {
		auth, err := rData.Auth()
		if err != nil {
			return nil, err
		}

		data["auth"] = auth
	}
	return data, nil
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

func (r *RegistryData) GetUserNameAndPasswordFromAuth() (string, string, error) {
	encodedAuth, err := r.Auth()
	if err != nil {
		return "", "", err
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(encodedAuth)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode auth: %v", err)
	}

	decodedAuth := string(decodedBytes)
	parts := strings.SplitN(decodedAuth, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid auth format, expected 'username:password'")
	}

	username := parts[0]
	password := parts[1]
	return username, password, nil
}

func (r *Registry) EmbeddedRegistryModuleMode() string {
	switch r.ModeSpecificFields.(type) {
	case ProxyModeRegistryData:
		return RegistryModeProxy
	case DetachedModeRegistryData:
		return RegistryModeDetached
	}
	return RegistryModeDirect
}

func (r *Registry) IsDirect() bool {
	mode := r.EmbeddedRegistryModuleMode()
	if mode == "" || mode == RegistryModeDirect {
		return true
	}
	return false
}

func (r *Registry) IsProxy() (*ProxyModeRegistryData, bool) {
	data, ok := r.ModeSpecificFields.(ProxyModeRegistryData)
	if ok {
		return &data, true
	}
	return nil, false
}

func (r *Registry) IsDetached() (*DetachedModeRegistryData, bool) {
	data, ok := r.ModeSpecificFields.(DetachedModeRegistryData)
	if ok {
		return &data, true
	}
	return nil, false
}

func (r *Registry) Mode() string {
	if r.IsDirect() {
		return RegistryModeDirect
	}
	return RegistryModeIndirect
}

func (r Registry) DeepCopy() Registry {
	var modeSpecificFieldsCopy interface{}
	switch r.ModeSpecificFields.(type) {
	case ProxyModeRegistryData:
		modeSpecificFields := r.ModeSpecificFields.(ProxyModeRegistryData)
		modeSpecificFieldsCopy = modeSpecificFields.DeepCopy()
	case DetachedModeRegistryData:
		modeSpecificFields := r.ModeSpecificFields.(DetachedModeRegistryData)
		modeSpecificFieldsCopy = modeSpecificFields.DeepCopy()
	}
	return Registry{
		Data:               r.Data,
		ModeSpecificFields: modeSpecificFieldsCopy,
	}
}

func (r Registry) ConvertToMap() (map[string]interface{}, error) {
	log.DebugF("registry: %v\n", r)

	mapData, err := r.Data.ConvertToMap()
	if err != nil {
		return nil, err
	}
	mapData["registryMode"] = r.Mode()
	mapData["embeddedRegistryModuleMode"] = r.EmbeddedRegistryModuleMode()

	switch r.ModeSpecificFields.(type) {
	case ProxyModeRegistryData:
		modeSpecificFields := r.ModeSpecificFields.(ProxyModeRegistryData)
		mapData["internalRegistryAccess"] = modeSpecificFields.InternalRegistryAccess.ConvertToMap()
		mapData["upstreamRegistry"], err = modeSpecificFields.UpstreamRegistryData.ConvertToMap()
		mapData["ttl"] = modeSpecificFields.TTL.String()
		if err != nil {
			return nil, err
		}
	case DetachedModeRegistryData:
		modeSpecificFields := r.ModeSpecificFields.(DetachedModeRegistryData)
		mapData["internalRegistryAccess"] = modeSpecificFields.InternalRegistryAccess.ConvertToMap()
	}
	return mapData, nil
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
