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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	otattribute "go.opentelemetry.io/otel/attribute"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/initconfig"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/minget"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
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

	CloudProviderVars *CloudProviderVars `json:"-"`
	// Operation propagates the dhctl entry point (bootstrap/converge/destroy/
	// check) to the provider preparator. It is intentionally serialised
	// (json:"operation") so a JSON round-trip through dhctl-server RPC or a
	// state-cache fallback preserves it. MarshalConfig clears it before
	// emitting tfvars, since the terraform layer does not consume it.
	Operation string `json:"operation,omitempty"`

	ClusterDomain string `json:"-"`

	ProviderClusterConfig map[string]json.RawMessage `json:"providerClusterConfiguration,omitempty"`
	StaticClusterConfig   map[string]json.RawMessage `json:"staticClusterConfiguration,omitempty"`

	VersionMap                map[string]any          `json:"-"`
	Images                    imagesDigests           `json:"-"`
	Registry                  registry.Config         `json:"-"`
	UUID                      string                  `json:"clusterUUID,omitempty"`
	InstallerVersion          string                  `json:"-"`
	ResourcesYAML             string                  `json:"-"`
	ResourceManagementTimeout string                  `json:"resourceManagementTimeout,omitempty"`
	ClusterMasterEndpoints    []ClusterMasterEndpoint `json:"-"`
	DownloadRootDir           string                  `json:"-"`
	DownloadCacheDir          string                  `json:"-"`
	ShowProgress              bool                    `json:"-"`

	// VersionFilePath is the absolute path to the deckhouse version file
	// embedded in the installer image. Required by LoadInstallerVersion and
	// DeckhouseInstaller.GetImageTag.
	VersionFilePath string `json:"-"`
}

type imagesDigests map[string]map[string]any

type ClusterMasterEndpoint struct {
	Address                string `json:"address" yaml:"address"`
	KubeAPIPort            int    `json:"kubeApiPort,omitempty" yaml:"kubeApiPort,omitempty"`
	RPPServerPort          int    `json:"rppServerPort,omitempty" yaml:"rppServerPort,omitempty"`
	RPPBootstrapServerPort int    `json:"rppBootstrapServerPort,omitempty" yaml:"rppBootstrapServerPort,omitempty"`
}

const (
	defaultClusterMasterAddress                = "127.0.0.1"
	defaultClusterMasterRPPServerPort          = 5444
	defaultClusterMasterRPPBootstrapServerPort = 4282
)

func validateAndPrepareMetaConfig(ctx context.Context, preparatorProvider MetaConfigPreparatorProvider, m *MetaConfig) (*MetaConfig, error) {
	ctx, span := telemetry.StartSpan(ctx, "validateAndPrepareMetaConfig")
	defer span.End()

	providerPreparator := preparatorProvider(m.ProviderName, m.DownloadRootDir)
	providerInput := m.buildProviderInput()

	span.SetAttributes(
		otattribute.String("provider.name", m.ProviderName),
		otattribute.String("provider.layout", m.Layout),
		otattribute.String("provider.clusterPrefix", m.ClusterPrefix),
		otattribute.String("provider.operation", m.Operation),
		otattribute.String("provider.downloadRootDir", m.DownloadRootDir),
		otattribute.Int("provider.input.providerClusterConfigKeys", len(providerInput.ProviderClusterConfig)),
	)
	if cv := providerInput.CloudProviderVars; cv != nil {
		span.SetAttributes(
			otattribute.Int("provider.input.settingsKeys", len(cv.Settings)),
			otattribute.Int("provider.input.nodeGroupsCount", len(cv.NodeGroups)),
			otattribute.Int("provider.input.instanceClassesCount", len(cv.InstanceClasses)),
			otattribute.Int("provider.input.secretsCount", len(cv.Secrets)),
		)
	}

	if err := providerPreparator.Validate(ctx, providerInput); err != nil {
		return nil, err
	}
	span.AddEvent("provider validated")

	result, err := providerPreparator.Prepare(ctx, providerInput)
	if err != nil {
		return nil, err
	}
	span.AddEvent("provider prepared")
	span.SetAttributes(otattribute.Int("provider.output.providerClusterConfigKeys", len(result.ProviderClusterConfig)))

	if result.Vars != nil {
		m.CloudProviderVars = result.Vars
		span.SetAttributes(
			otattribute.Int("provider.output.nodeGroupsCount", len(result.Vars.NodeGroups)),
			otattribute.Int("provider.output.instanceClassesCount", len(result.Vars.InstanceClasses)),
			otattribute.Int("provider.output.secretsCount", len(result.Vars.Secrets)),
		)
	}
	if len(result.ProviderClusterConfig) > 0 && m.ProviderClusterConfig == nil {
		m.ProviderClusterConfig = make(map[string]json.RawMessage, len(result.ProviderClusterConfig))
	}
	for k, v := range result.ProviderClusterConfig {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal provider cluster config key %q: %w", k, err)
		}
		m.ProviderClusterConfig[k] = raw
	}

	if len(result.ProviderClusterConfig) > 0 {
		if err := m.validateMutatedProviderClusterConfig(); err != nil {
			return nil, err
		}
	}

	// Re-extract typed fields: the preparator may have mutated PCC.
	if err := m.extractProviderClusterFields(); err != nil {
		return nil, err
	}

	return m, nil
}

// validateMutatedProviderClusterConfig re-validates ProviderClusterConfig
// against the provider schema after a preparator mutated it, so a buggy
// provider binary cannot inject an invalid configuration into tfvars.
func (m *MetaConfig) validateMutatedProviderClusterConfig() error {
	var index SchemaIndex
	_ = json.Unmarshal(m.ProviderClusterConfig["kind"], &index.Kind)
	_ = json.Unmarshal(m.ProviderClusterConfig["apiVersion"], &index.Version)
	if !index.IsValid() {
		return nil
	}

	doc, err := json.Marshal(m.ProviderClusterConfig)
	if err != nil {
		return fmt.Errorf("marshal mutated provider cluster configuration: %w", err)
	}
	if err := NewSchemaStore(nil).ValidateWithIndex(&index, &doc); err != nil && !errors.Is(err, ErrSchemaNotFound) {
		return fmt.Errorf("provider %s preparator mutated provider cluster configuration into an invalid state: %w", m.ProviderName, err)
	}
	return nil
}

var deprecatedClusterConfigFields = []struct {
	field   string
	message string
}{
	{
		field:   "encryptionAlgorithm",
		message: "Please migrate it to the \"control-plane-manager\" ModuleConfig: spec.settings.encryptionAlgorithm.",
	},
}

func (m *MetaConfig) warnDeprecatedClusterConfigFields() {
	for _, d := range deprecatedClusterConfigFields {
		if _, ok := m.ClusterConfig[d.field]; ok {
			log.InteractiveWarnLn("=================================================================")
			log.InteractiveWarnLn(fmt.Sprintf("DEPRECATED: %q in ClusterConfiguration is deprecated.", d.field))
			log.InteractiveWarnLn(d.message)
			log.InteractiveWarnLn("Support for this field in ClusterConfiguration will be removed in a future release.")
			log.InteractiveWarnLn("=================================================================")
		}
	}
}

// Prepare extracts all necessary information from raw json messages to the root structure
func (m *MetaConfig) Prepare(ctx context.Context, preparatorProvider MetaConfigPreparatorProvider) (*MetaConfig, error) {
	if len(m.ClusterConfig) > 0 {
		m.warnDeprecatedClusterConfigFields()
		if err := json.Unmarshal(m.ClusterConfig["clusterType"], &m.ClusterType); err != nil {
			return nil, fmt.Errorf("unable to parse cluster type from cluster configuration: %v", err)
		}

		var serviceSubnet string
		if err := json.Unmarshal(m.ClusterConfig["serviceSubnetCIDR"], &serviceSubnet); err != nil {
			return nil, fmt.Errorf("unable to unmarshal service subnet CIDR from cluster configuration: %v", err)
		}
		m.ClusterDNSAddress = getDNSAddress(serviceSubnet)

		if err := json.Unmarshal(m.ClusterConfig["clusterDomain"], &m.ClusterDomain); err != nil {
			return nil, fmt.Errorf("unable to unmarshal cluster domain from cluster configuration: %w", err)
		}
	}

	if len(m.InitClusterConfig) > 0 {
		if err := json.Unmarshal(m.InitClusterConfig["deckhouse"], &m.DeckhouseConfig); err != nil {
			return nil, fmt.Errorf("unable to unmarshal deckhouse configuration: %v", err)
		}
	}

	// Prepare registry config
	if err := m.prepareRegistry(); err != nil {
		return nil, fmt.Errorf("unable to initialize registry config: %w", err)
	}

	if m.ClusterType != CloudClusterType {
		return validateAndPrepareMetaConfig(ctx, preparatorProvider, m)
	}

	if err := m.prepareProviderName(); err != nil {
		return nil, err
	}
	cloudSpec, err := m.clusterCloudSpec()
	if err != nil {
		return nil, err
	}
	m.ClusterPrefix = cloudSpec.Prefix

	if err := m.extractProviderClusterFields(); err != nil {
		return nil, err
	}

	if m.CloudProviderVars == nil && m.ResourcesYAML != "" {
		cv, err := ParseResourcesYAML(m.ResourcesYAML)
		if err != nil {
			return nil, fmt.Errorf("parse cloud provider resources: %w", err)
		}
		m.CloudProviderVars = cv
	}

	if err := m.applyCloudProviderModuleSettings(); err != nil {
		return nil, err
	}

	return validateAndPrepareMetaConfig(ctx, preparatorProvider, m)
}

// extractProviderClusterFields populates the typed Layout, MasterNodeGroupSpec
// and TerraNodeGroupSpecs from PCC (legacy flow), falling back to
// CloudProviderVars.NodeGroups when PCC is empty (mc-flow). For providers that
// require PCC a missing field fails fast instead of running converge with
// zeroed typed fields.
func (m *MetaConfig) extractProviderClusterFields() error {
	pccRequired := ProviderRequiresClusterConfig(m.ProviderName)
	pccPresent := len(m.ProviderClusterConfig) > 0

	if raw, ok := m.ProviderClusterConfig["layout"]; ok && len(raw) > 0 {
		var layout string
		if err := json.Unmarshal(raw, &layout); err != nil {
			return fmt.Errorf("unmarshal layout from provider cluster configuration: %w", err)
		}
		if layout != "" {
			m.Layout = strcase.ToKebab(layout)
		}
	} else if pccRequired && pccPresent {
		return fmt.Errorf("provider cluster configuration for %q is missing required field %q", m.ProviderName, "layout")
	}
	if raw, ok := m.ProviderClusterConfig["masterNodeGroup"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &m.MasterNodeGroupSpec); err != nil {
			return fmt.Errorf("unmarshal master node group from provider cluster configuration: %w", err)
		}
	} else if pccRequired && pccPresent {
		return fmt.Errorf("provider cluster configuration for %q is missing required field %q", m.ProviderName, "masterNodeGroup")
	}
	if raw, ok := m.ProviderClusterConfig["nodeGroups"]; ok && len(raw) > 0 {
		m.TerraNodeGroupSpecs = []TerraNodeGroupSpec{}
		if err := json.Unmarshal(raw, &m.TerraNodeGroupSpecs); err != nil {
			return fmt.Errorf("unmarshal node groups from provider cluster configuration: %w", err)
		}
	}
	// "nodeGroups" is not required even for whitelisted providers: a
	// master-only cluster is legitimate.

	// mc-flow: derive replica counts from cluster NodeGroups, otherwise the
	// zeroed typed fields misroute converge into "decrease to 0" logic.
	applyNodeGroupReplicasFromCloudProviderVars(m)
	return nil
}

// masterNodeGroupName is the canonical control-plane NodeGroup name; renaming
// it is not supported in Deckhouse.
const masterNodeGroupName = "master"

func applyNodeGroupReplicasFromCloudProviderVars(m *MetaConfig) {
	if m.CloudProviderVars == nil {
		return
	}
	if _, hasMaster := m.CloudProviderVars.NodeGroups[masterNodeGroupName]; hasMaster && m.MasterNodeGroupSpec.Replicas == 0 {
		if r := nodeGroupReplicas(m.CloudProviderVars.NodeGroups, masterNodeGroupName); r > 0 {
			m.MasterNodeGroupSpec.Replicas = r
		}
	}
	if len(m.TerraNodeGroupSpecs) == 0 && len(m.CloudProviderVars.NodeGroups) > 0 {
		// Iterate over sorted names so the order of TerraNodeGroupSpecs
		// is reproducible. Map iteration is randomised by the Go runtime;
		// downstream state-hash comparisons see spurious drift otherwise.
		//
		// All non-master CloudPermanent NodeGroups (already filtered by
		// IsCloudPermanentNodeGroup before reaching here) are emitted as
		// TerraNodeGroupSpecs so converge keeps managing them.
		for _, name := range sortedKeys(m.CloudProviderVars.NodeGroups) {
			if name == masterNodeGroupName {
				continue
			}
			ng := m.CloudProviderVars.NodeGroups[name]
			r := nodeGroupReplicas(m.CloudProviderVars.NodeGroups, name)
			nodeTemplate, _ := nestedMap(ng, "spec", "nodeTemplate")
			m.TerraNodeGroupSpecs = append(m.TerraNodeGroupSpecs, TerraNodeGroupSpec{
				Name:         name,
				Replicas:     r,
				NodeTemplate: nodeTemplate,
			})
		}
	}
}

// nodeGroupReplicas derives the replica count for a CloudPermanent NodeGroup
// from spec.cloudInstances.minPerZone. CloudPermanent NGs always set this
// field; spec.staticInstances.count is kept as a defensive fallback in case
// the upstream IsCloudPermanentNodeGroup filter is ever relaxed. Returning
// zero is interpreted by MasterNodeGroupController as "scale to zero", so
// the caller must explicitly guard the master NG against that outcome.
func nodeGroupReplicas(ngs map[string]map[string]interface{}, name string) int {
	ng, ok := ngs[name]
	if !ok {
		return 0
	}
	if ci, ok := nestedMap(ng, "spec", "cloudInstances"); ok {
		if r := toPositiveInt(ci["minPerZone"]); r > 0 {
			return r
		}
	}
	if si, ok := nestedMap(ng, "spec", "staticInstances"); ok {
		if r := toPositiveInt(si["count"]); r > 0 {
			return r
		}
	}
	return 0
}

func sortedKeys(m map[string]map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toPositiveInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		if n > 0 {
			return int(n)
		}
	case int64:
		if n > 0 {
			return int(n)
		}
	case int:
		if n > 0 {
			return n
		}
	}
	return 0
}

func nestedMap(obj map[string]interface{}, path ...string) (map[string]interface{}, bool) {
	cur := obj
	for _, p := range path {
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// cloudProviderModuleSettings is the typed view of the fields dhctl reads
// out of the cloud-provider-<name> ModuleConfig in the ModuleConfig-only flow.
type cloudProviderModuleSettings struct {
	Nodes struct {
		Parameters struct {
			Layout string `json:"layout"`
		} `json:"parameters"`
	} `json:"nodes"`
}

// applyCloudProviderModuleSettings fills CloudProviderVars.Settings from the
// cloud-provider-<name> ModuleConfig and extracts the typed Layout. Used on
// bootstrap-from-file when <Provider>ClusterConfiguration is not supplied —
// the external preparator and terraform-modules read settings from the MC
// instead.
//
// CloudProviderVars.Settings holds the *full* ModuleConfig object (apiVersion,
// kind, metadata, spec.{version, settings, enabled}), not just spec.settings.
// terraform-modules/migration reads var.settings.spec.version and
// var.settings.spec.settings.*; the DVP external validator forwards
// input.ModuleConfig back as result.vars.settings unchanged.
//
// Users can supply multiple ModuleConfigs with the same name (e.g. a base v1
// "enabled: true" entry plus a v2 overlay with the actual settings). We pick
// the *last* entry that carries non-empty Spec.Settings so that overlays win.
func (m *MetaConfig) applyCloudProviderModuleSettings() error {
	name := CloudProviderModuleName(m.ProviderName)
	var picked *ModuleConfig
	for _, mc := range m.ModuleConfigs {
		if mc.GetName() == name && len(mc.Spec.Settings) > 0 {
			picked = mc
		}
	}
	if picked == nil {
		return nil
	}

	mcMap, err := moduleConfigToMap(picked)
	if err != nil {
		return err
	}
	if m.CloudProviderVars == nil {
		m.CloudProviderVars = &CloudProviderVars{}
	}
	if len(m.CloudProviderVars.Settings) == 0 {
		m.CloudProviderVars.Settings = mcMap
	}

	if m.Layout == "" {
		if layout := picked.Spec.layoutString(); layout != "" {
			m.Layout = strcase.ToKebab(layout)
		}
	}
	return nil
}

func moduleConfigToMap(mc *ModuleConfig) (map[string]interface{}, error) {
	raw, err := json.Marshal(mc)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud-provider module config: %w", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal cloud-provider module config: %w", err)
	}
	return out, nil
}

func (s ModuleConfigSpec) layoutString() string {
	raw, err := json.Marshal(s.Settings)
	if err != nil {
		return ""
	}
	var typed cloudProviderModuleSettings
	if err := json.Unmarshal(raw, &typed); err != nil {
		return ""
	}
	return typed.Nodes.Parameters.Layout
}

func (m *MetaConfig) clusterCloudSpec() (ClusterConfigCloudSpec, error) {
	var cloud ClusterConfigCloudSpec
	if err := json.Unmarshal(m.ClusterConfig["cloud"], &cloud); err != nil {
		return cloud, fmt.Errorf("parse cloud spec: %w", err)
	}
	return cloud, nil
}

func (m *MetaConfig) prepareProviderName() error {
	cloud, err := m.clusterCloudSpec()
	if err != nil {
		return err
	}
	if cloud.Provider == "" {
		return fmt.Errorf("unmarshal cloud section from cluster configuration: provider is empty")
	}
	m.ProviderName = strings.ToLower(cloud.Provider)
	m.OriginalProviderName = cloud.Provider
	return nil
}

func (m *MetaConfig) buildProviderInput() ProviderInput {
	return ProviderInput{
		ProviderName:          m.ProviderName,
		ClusterPrefix:         m.ClusterPrefix,
		Layout:                m.Layout,
		Operation:             m.Operation,
		ProviderClusterConfig: m.ProviderClusterConfig,
		CloudProviderVars:     m.CloudProviderVars,
	}
}

func (m *MetaConfig) prepareRegistry() error {
	var (
		initConfig        *initconfig.Config
		deckhouseSettings *moduleconfig.DeckhouseSettings
		defaultCRI        registry_const.CRIType
	)

	// Init config
	if len(m.InitClusterConfig) > 0 {
		rawJSON, err := json.Marshal(m.InitClusterConfig)
		if err != nil {
			return err
		}
		initConfig, err = registry.ParseJSONInitConfig(rawJSON)
		if err != nil {
			return err
		}
	}

	// Deckhouse mc
	if mc := m.FindModuleConfig("deckhouse"); mc != nil {
		rawJSON, err := json.Marshal(mc)
		if err != nil {
			return err
		}
		deckhouseSettings, err = registry.ParseJSONDeckhouseMC(rawJSON)
		if err != nil {
			return err
		}
	}

	// Default CRI
	if rawCRI, exists := m.ClusterConfig["defaultCRI"]; exists {
		if err := json.Unmarshal(rawCRI, &defaultCRI); err != nil {
			return fmt.Errorf("get defaultCRI from cluster config: %w", err)
		}
	}

	registry, err := registry.NewConfigProvider(
		initConfig,
		deckhouseSettings,
	).Config(
		defaultCRI,
		m.IsStatic(),
	)

	if err == nil {
		m.Registry = registry
	}
	return err
}

func (m *MetaConfig) GetFullUUID() (string, error) {
	if m.UUID == "" {
		return "", fmt.Errorf("Unable to get full UUID for provider '%s/%s'. It is empty", m.ClusterPrefix, m.ProviderName)
	}
	return m.UUID, nil
}

func (m *MetaConfig) GetTerraNodeGroups() []TerraNodeGroupSpec {
	return m.TerraNodeGroupSpecs
}

func (m *MetaConfig) GetClusterDomain() string {
	return m.ClusterDomain
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

// FindProviderModuleConfig returns the cloud-provider-<name> ModuleConfig
// when present, or nil. Used to detect whether the cluster is on the new
// mc-flow provider format.
func (m *MetaConfig) FindProviderModuleConfig() *ModuleConfig {
	if m == nil || m.ProviderName == "" {
		return nil
	}
	return m.FindModuleConfig(CloudProviderModuleName(m.ProviderName))
}

// HasProviderModuleConfig reports whether the cluster carries a
// cloud-provider-<name> ModuleConfig (the new mc-flow provider format).
func (m *MetaConfig) HasProviderModuleConfig() bool {
	return m.FindProviderModuleConfig() != nil
}

// HasLegacyProviderConfig reports whether the cluster carries a non-empty
// d8-provider-cluster-configuration payload (the legacy provider format).
func (m *MetaConfig) HasLegacyProviderConfig() bool {
	return m != nil && len(m.ProviderClusterConfig) > 0
}

func (m *MetaConfig) ExtractMasterNodeGroupStaticSettings() map[string]any {
	static := make(map[string]any)

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
func (m *MetaConfig) NodeGroupManifest(terraNodeGroup TerraNodeGroupSpec) map[string]any {
	if terraNodeGroup.NodeTemplate == nil {
		terraNodeGroup.NodeTemplate = make(map[string]any)
	}
	return map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]any{
			"name": terraNodeGroup.Name,
		},
		"spec": map[string]any{
			"nodeType": "CloudPermanent",
			"disruptions": map[string]any{
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

// MarshalConfig serializes the MetaConfig to JSON. When CloudProviderVars is
// set, its fields (settings, nodeGroups, instanceClasses, secrets) are added at
// the top level alongside clusterConfiguration/providerClusterConfiguration.
func (m *MetaConfig) MarshalConfig() []byte {
	type cfg struct {
		*MetaConfig
		Settings        map[string]interface{}            `json:"settings,omitempty"`
		NodeGroups      map[string]map[string]interface{} `json:"nodeGroups,omitempty"`
		InstanceClasses map[string]map[string]interface{} `json:"instanceClasses,omitempty"`
		Secrets         map[string]map[string]interface{} `json:"secrets,omitempty"`
	}

	newM := m.DeepCopy()
	newM.StaticClusterConfig = nil
	// Operation is not part of the terraform input contract — strip it
	// before the tfvars JSON reaches OpenTofu/Terraform.
	newM.Operation = ""

	wrap := cfg{MetaConfig: newM}
	if cv := m.CloudProviderVars; cv != nil {
		wrap.Settings = cv.Settings
		wrap.NodeGroups = cv.NodeGroups
		wrap.InstanceClasses = cv.InstanceClasses
		wrap.Secrets = cv.Secrets
	}

	data, _ := json.Marshal(wrap)
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

func resolveKubernetesVersion(v string) string {
	if v == "Automatic" {
		return DefaultKubernetesVersion
	}
	return v
}

func (m *MetaConfig) ClusterConfigMap() (map[string]interface{}, error) {
	raw := m.ClusterConfig
	out := make(map[string]interface{}, len(raw))
	for k, v := range raw {
		var a any
		if err := json.Unmarshal(v, &a); err != nil {
			return nil, fmt.Errorf("unmarshal ClusterConfiguration field %q: %w", k, err)
		}
		out[k] = a
	}
	if v, _ := out["kubernetesVersion"].(string); v != "" {
		out["kubernetesVersion"] = resolveKubernetesVersion(v)
	}
	return out, nil
}

func (m *MetaConfig) ConfigForBashibleBundleTemplate(nodeIP string) (map[string]interface{}, error) {
	data := make(map[string]interface{}, len(m.ClusterConfig))

	for key, value := range m.ClusterConfig {
		var t any
		err := json.Unmarshal(value, &t)
		if err != nil {
			return nil, fmt.Errorf("cluster config unmarshal: %v", err)
		}
		data[key] = t
	}

	if data["kubernetesVersion"] == "Automatic" {
		data["kubernetesVersion"] = DefaultKubernetesVersion
	}

	clusterBootstrap := map[string]any{
		"clusterDomain":     data["clusterDomain"],
		"clusterDNSAddress": m.ClusterDNSAddress,
	}

	if nodeIP != "" {
		clusterBootstrap["cloud"] = map[string]any{"nodeIP": nodeIP}
	}

	nodeGroup := map[string]any{
		"name":     "master",
		"nodeType": "CloudPermanent",
		"cloudInstances": map[string]any{
			"classReference": map[string]string{
				"name": "master",
			},
		},
	}

	if m.ClusterType == StaticClusterType {
		nodeGroup["nodeType"] = "Static"
		nodeGroup["static"] = m.ExtractMasterNodeGroupStaticSettings()
	}

	configForBashibleBundleTemplate := make(map[string]any)
	maps.Copy(configForBashibleBundleTemplate, m.VersionMap)

	configForBashibleBundleTemplate["runType"] = "ClusterBootstrap"

	if m.ClusterType == CloudClusterType {
		configForBashibleBundleTemplate["provider"] = m.ProviderName
	}

	configForBashibleBundleTemplate["cri"] = data["defaultCRI"]
	configForBashibleBundleTemplate["kubernetesVersion"] = data["kubernetesVersion"]
	configForBashibleBundleTemplate["nodeGroup"] = nodeGroup
	configForBashibleBundleTemplate["clusterBootstrap"] = clusterBootstrap
	configForBashibleBundleTemplate["proxy"] = make(map[string]any)
	if data["proxy"] != nil {
		proxyData, err := m.EnrichProxyData()
		if err != nil {
			return nil, err
		}
		if proxyData != nil {
			configForBashibleBundleTemplate["proxy"] = proxyData
		}
	}

	// Registry
	registryContext, err := m.Registry.
		Manifest().
		BashibleContext(registry.GeneratePKI)
	if err != nil {
		return nil, fmt.Errorf("create registry bashible context: %s", err)
	}
	configForBashibleBundleTemplate["registry"] = registryContext.ToMap()

	if tag := m.deckhouseImageTag(); tag != "" && registryContext.ImagesBase != "" {
		configForBashibleBundleTemplate["deckhouseImageRef"] = fmt.Sprintf("%s:%s", registryContext.ImagesBase, tag)
	}

	images := m.Images
	configForBashibleBundleTemplate["images"] = images.ConvertToMap()
	clusterMasterEndpoints := m.clusterMasterEndpointsBashibleContext()
	configForBashibleBundleTemplate["clusterMasterEndpoints"] = clusterMasterEndpoints
	configForBashibleBundleTemplate["clusterMasterKubeAPIEndpoints"] = clusterMasterEndpointAddresses(clusterMasterEndpoints, "kubeApiPort")
	configForBashibleBundleTemplate["clusterMasterRPPAddresses"] = clusterMasterEndpointAddresses(clusterMasterEndpoints, "rppServerPort")
	configForBashibleBundleTemplate["clusterMasterRPPBootstrapAddresses"] = clusterMasterEndpointAddresses(clusterMasterEndpoints, "rppBootstrapServerPort")

	mingetBytes, err := minget.Bytes()
	if err != nil {
		return nil, fmt.Errorf("get minget bytes: %w", err)
	}
	configForBashibleBundleTemplate["mingetB64"] = base64.StdEncoding.EncodeToString(mingetBytes)

	return configForBashibleBundleTemplate, nil
}

// NodeGroupConfig returns values for infrastructure utility to order master node
// or static node. When CloudProviderVars is set, its fields are added at the top
// level.
func (m *MetaConfig) NodeGroupConfig(nodeGroupName string, nodeIndex int, cloudConfig string) []byte {
	result := map[string]any{
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

	if cv := m.CloudProviderVars; cv != nil {
		if cv.Settings != nil {
			result["settings"] = cv.Settings
		}
		if cv.NodeGroups != nil {
			result["nodeGroups"] = cv.NodeGroups
		}
		if cv.InstanceClasses != nil {
			result["instanceClasses"] = cv.InstanceClasses
		}
		if cv.Secrets != nil {
			result["secrets"] = cv.Secrets
		}
	}

	data, _ := json.Marshal(result)
	return data
}

func (m *MetaConfig) CachePath() string {
	return fmt.Sprintf("%s-%s-terraform-state-cache", m.ClusterPrefix, m.ProviderName)
}

func (m *MetaConfig) DeepCopy() *MetaConfig {
	out := *m
	out.Registry = *m.Registry.DeepCopy()
	out.ClusterConfig = cloneMap(m.ClusterConfig)
	out.InitClusterConfig = cloneMap(m.InitClusterConfig)
	out.ProviderClusterConfig = cloneMap(m.ProviderClusterConfig)
	out.StaticClusterConfig = cloneMap(m.StaticClusterConfig)
	out.CloudProviderVars = cloneCloudProviderVars(m.CloudProviderVars)
	out.ModuleConfigs = cloneModuleConfigs(m.ModuleConfigs)
	out.VersionMap = cloneMap(m.VersionMap)
	out.Images = cloneNestedMap(m.Images)
	if m.TerraNodeGroupSpecs != nil {
		out.TerraNodeGroupSpecs = make([]TerraNodeGroupSpec, len(m.TerraNodeGroupSpecs))
		copy(out.TerraNodeGroupSpecs, m.TerraNodeGroupSpecs)
	}
	if m.ClusterMasterEndpoints != nil {
		out.ClusterMasterEndpoints = make([]ClusterMasterEndpoint, len(m.ClusterMasterEndpoints))
		copy(out.ClusterMasterEndpoints, m.ClusterMasterEndpoints)
	}
	return &out
}

// cloneMap is a thin nil-preserving wrapper over maputil.Clone — DeepCopy
// callers rely on nil staying nil (matters for json.Marshal omitempty).
func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	if in == nil {
		return nil
	}
	return maputil.Clone(in)
}

func cloneNestedMap[K comparable, V any](in map[K]map[K]V) map[K]map[K]V {
	if in == nil {
		return nil
	}
	out := make(map[K]map[K]V, len(in))
	for k, v := range in {
		out[k] = cloneMap(v)
	}
	return out
}

func cloneCloudProviderVars(v *CloudProviderVars) *CloudProviderVars {
	if v == nil {
		return nil
	}
	return &CloudProviderVars{
		Settings:        cloneMap(v.Settings),
		NodeGroups:      cloneNestedMap(v.NodeGroups),
		InstanceClasses: cloneNestedMap(v.InstanceClasses),
		Secrets:         cloneNestedMap(v.Secrets),
	}
}

func cloneModuleConfigs(in []*ModuleConfig) []*ModuleConfig {
	if in == nil {
		return nil
	}
	out := make([]*ModuleConfig, len(in))
	for i, mc := range in {
		if mc == nil {
			continue
		}
		cp := *mc
		cp.Spec.Settings = SettingsValues(cloneMap(map[string]interface{}(mc.Spec.Settings)))
		if mc.Spec.Enabled != nil {
			enabled := *mc.Spec.Enabled
			cp.Spec.Enabled = &enabled
		}
		out[i] = &cp
	}
	return out
}

func (m *MetaConfig) clusterMasterEndpointsBashibleContext() []map[string]any {
	clusterMasterEndpoints := m.effectiveClusterMasterEndpoints()
	endpoints := make([]map[string]any, 0, len(clusterMasterEndpoints))

	for _, endpoint := range clusterMasterEndpoints {
		item := map[string]any{
			"address": endpoint.Address,
		}

		if endpoint.KubeAPIPort != 0 {
			item["kubeApiPort"] = endpoint.KubeAPIPort
		}
		if endpoint.RPPServerPort != 0 {
			item["rppServerPort"] = endpoint.RPPServerPort
		}
		if endpoint.RPPBootstrapServerPort != 0 {
			item["rppBootstrapServerPort"] = endpoint.RPPBootstrapServerPort
		}

		endpoints = append(endpoints, item)
	}

	return endpoints
}

func clusterMasterEndpointAddresses(endpoints []map[string]any, portName string) []string {
	addresses := make([]string, 0, len(endpoints))

	for _, endpoint := range endpoints {
		address, ok := endpoint["address"].(string)
		if !ok || address == "" {
			continue
		}

		port, ok := endpoint[portName].(int)
		if !ok || port == 0 {
			continue
		}

		addresses = append(addresses, fmt.Sprintf("%s:%d", address, port))
	}

	return addresses
}

func (m *MetaConfig) effectiveClusterMasterEndpoints() []ClusterMasterEndpoint {
	if len(m.ClusterMasterEndpoints) > 0 {
		return m.ClusterMasterEndpoints
	}

	return []ClusterMasterEndpoint{
		{
			Address:                defaultClusterMasterAddress,
			RPPServerPort:          defaultClusterMasterRPPServerPort,
			RPPBootstrapServerPort: defaultClusterMasterRPPBootstrapServerPort,
		},
	}
}

func (m *MetaConfig) LoadVersionMap(filename string) error {
	versionMap := make(map[string]any)

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

func (m *MetaConfig) EnrichProxyData() (map[string]any, error) {
	type proxy struct {
		HTTPProxy  string   `json:"httpProxy" yaml:"httpProxy"`
		HTTPSProxy string   `json:"httpsProxy" yaml:"httpsProxy"`
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

	ret := make(map[string]any)
	if p.HTTPProxy != "" {
		ret["httpProxy"] = p.HTTPProxy
	}
	if p.HTTPSProxy != "" {
		ret["httpsProxy"] = p.HTTPSProxy
	}
	ret["noProxy"] = p.NoProxy

	return ret, nil
}

func (m *MetaConfig) LoadImagesDigests() error {
	imagesDigests, err := digests.GetAllDigests()
	if err != nil {
		return fmt.Errorf("Cannot get image digests: %w", err)
	}

	m.Images = imagesDigests

	return nil
}

// FindModuleConfig
// if not found returns nil
func (m *MetaConfig) FindModuleConfig(module string) *ModuleConfig {
	if len(m.ModuleConfigs) == 0 {
		return nil
	}

	for _, moduleConfig := range m.ModuleConfigs {
		if moduleConfig.Name == module {
			return moduleConfig
		}
	}

	return nil
}

// deckhouseImageTag returns the deckhouse-controller image tag for the bashible
// prefetch: semver from the installer's version file, falling back to DevBranch.
func (m *MetaConfig) deckhouseImageTag() string {
	if m.VersionFilePath != "" {
		if tag, ok := ReadVersionTagFromInstallerContainer(m.VersionFilePath, m.DownloadRootDir); ok {
			return tag
		}
	}
	return m.DeckhouseConfig.DevBranch
}

func (m *MetaConfig) LoadInstallerVersion() error {
	rawFile, err := os.ReadFile(m.VersionFilePath)
	if err != nil {
		// fallback to the unpacked deckhouse image location
		fallbackPath := filepath.Join(m.DownloadRootDir, "deckhouse", "version")
		rawFile, err = os.ReadFile(fallbackPath)
		if err != nil {
			return fmt.Errorf("could not read both %s and %s: %w", m.VersionFilePath, fallbackPath, err)
		}
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
		log.DebugLn("serviceSubnetCIDR is not a valid CIDR (should be validated with the openapi schema)")
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

func (i *imagesDigests) ConvertToMap() map[string]any {
	res := make(map[string]any)
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
