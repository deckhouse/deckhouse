/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "github.com/deckhouse/deckhouse/ee/modules/030-cloud-provider-vcd/hooks/internal/v1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: vcd_cluster_configuration ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
cloudProviderVcd:
  internal: {}
`
		hybridValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    username: module-user
    password: module-password
    insecure: false
  organization: module-org
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  sshPublicKey: module-ssh-key
  metadata:
    env: test
    owner: qa
  internal:
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: VCDCloudProviderDiscoveryData
      zones:
      - zone-a
      - zone-b
`
		emptyProviderValues = `
global:
  discovery: {}
cloudProviderVcd:
  internal: {}
  provider: {}
  metadata:
    from: module
`
		missingProviderServerValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    username: module-user
    password: module-password
  organization: module-org
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		missingOrganizationValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    username: module-user
    password: module-password
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		missingUsernameValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    password: module-password
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		missingPasswordValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    username: module-username
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		missingUserPassAndAPITokenValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		hasUserPassAndAPITokenValues = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    username: module-user
    password: module-password
    apiToken: module-api-token
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		hybridValuesWithoutDiscoveryData = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    username: module-user
    password: module-password
  organization: module-org
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal: {}
`
		partialInlineDiscoveryValuesWithoutDefaults = `
global:
  discovery: {}
cloudProviderVcd:
  provider:
    server: https://module.example.com/api
    username: module-user
    password: module-password
  organization: module-org
  virtualDataCenter: module-vdc
  virtualApplicationName: module-vapp
  mainNetwork: module-network
  internal:
    providerDiscoveryData:
      storageProfiles:
      - name: module-profile
        isEnabled: true
`
		mergeDiscoveryValues = `
global:
  discovery: {}
cloudProviderVcd:
  internal:
    providerDiscoveryData:
      zones:
      - module-zone
      storageProfiles:
      - name: module-profile
        isEnabled: true
`
		loadBalancerCurrentValues = `
global:
  discovery: {}
cloudProviderVcd:
  internal:
    providerDiscoveryData:
      loadBalancer:
        enabled: false
`
	)
	var (
		stateACloudDiscoveryData = `
{
   "apiVersion": "deckhouse.io/v1",
   "kind": "VCDCloudProviderDiscoveryData",
   "zones": ["default"]
}
`
		stateBCloudDiscoveryData = `
{
   "apiVersion": "deckhouse.io/v1",
   "kind": "VCDCloudProviderDiscoveryData",
   "sizingPolicies": ["secret-policy"],
   "internalNetworks": ["secret-network"],
   "zones": ["secret-zone"],
   "storageProfiles": [
     {
       "name": "secret-profile",
       "isEnabled": true
     }
   ],
   "vcdAPIVersion": "37.3",
   "vcdInstallationVersion": "10.5",
   "loadBalancer": {
     "enabled": true
   }
}
`
		stateAClusterConfiguration1 = `
apiVersion: deckhouse.io/v1
kind: VCDClusterConfiguration
sshPublicKey: "<SSH_PUBLIC_KEY>"
organization: My_Org
virtualDataCenter: My_Org
virtualApplicationName: vapp
layout: Standard
internalNetworkCIDR: 172.16.2.0/24
mainNetwork: internal
masterNodeGroup:
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    sizingPolicy: 4cpu8ram
    rootDiskSizeGb: 20
    etcdDiskSizeGb: 20
    storageProfile: nvme
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    rootDiskSizeGb: 20
    sizingPolicy: 16cpu32ram
    storageProfile: ssd
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
`
	)

	notEmptyProviderClusterConfigurationState := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

	notEmptyProviderClusterConfigurationStateWithRichDiscovery := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))

	// todo(31337Ghost) eliminate the following dirty hack after `ee` subdirectory will be merged to the root
	// Used to make dhctl config function able to validate `VsphereClusterConfiguration`.
	_ = os.Setenv("DHCTL_CLI_ADDITIONAL_SCHEMAS_PATHS", "/deckhouse/ee/modules/030-cloud-provider-vcd/candi/openapi")

	Context("Cluster without module configuration", func() {
		cfg := HookExecutionConfigInit(emptyValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(notEmptyProviderClusterConfigurationState))
			cfg.RunHook()
		})

		It("Should fill values from secret", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration").String()).To(MatchYAML(stateAClusterConfiguration1))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.metadata").String()).To(BeEmpty())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))
		})
	})

	Context("Cluster without secret and without module configuration", func() {
		cfg := HookExecutionConfigInit(emptyValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because provider section is required", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("provider section is required")))
		})
	})

	Context("Hybrid cluster with module configuration and without secret", func() {
		cfg := HookExecutionConfigInit(hybridValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should build provider cluster configuration from module config and preserve inline discovery data", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.server").String()).To(Equal("https://module.example.com/api"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.username").String()).To(Equal("module-user"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.password").String()).To(Equal("module-password"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.organization").String()).To(Equal("module-org"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.virtualDataCenter").String()).To(Equal("module-vdc"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.virtualApplicationName").String()).To(Equal("module-vapp"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.mainNetwork").String()).To(Equal("module-network"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.sshPublicKey").String()).To(Equal("module-ssh-key"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.metadata").String()).To(MatchYAML(`
env: test
owner: qa
`))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VCDCloudProviderDiscoveryData",
  "zones": ["zone-a", "zone-b"]
}
`))
		})
	})

	Context("Hybrid cluster with module configuration and bootstrap secret", func() {
		cfg := HookExecutionConfigInit(hybridValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(notEmptyProviderClusterConfigurationState))
			cfg.RunHook()
		})

		It("Should override secret values with module config and keep non-hybrid fields from secret", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.layout").String()).To(Equal("Standard"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.masterNodeGroup.replicas").Int()).To(Equal(int64(1)))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.server").String()).To(Equal("https://module.example.com/api"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.username").String()).To(Equal("module-user"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.password").String()).To(Equal("module-password"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.organization").String()).To(Equal("module-org"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.virtualDataCenter").String()).To(Equal("module-vdc"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.virtualApplicationName").String()).To(Equal("module-vapp"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.mainNetwork").String()).To(Equal("module-network"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.sshPublicKey").String()).To(Equal("module-ssh-key"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.metadata").String()).To(MatchYAML(`
env: test
owner: qa
`))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VCDCloudProviderDiscoveryData",
  "zones": ["zone-a", "zone-b"]
}
`))
		})
	})

	Context("Cluster with empty provider in module configuration and bootstrap secret", func() {
		cfg := HookExecutionConfigInit(emptyProviderValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(notEmptyProviderClusterConfigurationState))
			cfg.RunHook()
		})

		It("Should keep provider from secret and still patch metadata from module config", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.server").String()).To(Equal("<SERVER>"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.username").String()).To(Equal("<USERNAME>"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.provider.password").String()).To(Equal("<PASSWORD>"))
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerClusterConfiguration.metadata").String()).To(MatchYAML(`
from: module
`))
		})
	})

	Context("Cluster with module configuration and provider section without server", func() {
		cfg := HookExecutionConfigInit(missingProviderServerValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because provider.server is required", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("provider.server cannot be empty")))
		})
	})

	Context("Cluster with module configuration and missing organization", func() {
		cfg := HookExecutionConfigInit(missingOrganizationValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because organization is required", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("organization cannot be empty")))
		})
	})

	Context("Cluster with module configuration and missing username", func() {
		cfg := HookExecutionConfigInit(missingUsernameValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because provider.username and provider.password must be set together", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("both provider.username and provider.password should be set")))
		})
	})

	Context("Cluster with module configuration and missing password", func() {
		cfg := HookExecutionConfigInit(missingPasswordValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because provider.username and provider.password must be set together", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("both provider.username and provider.password should be set")))
		})
	})

	Context("Cluster with module configuration and without user/password and apiToken", func() {
		cfg := HookExecutionConfigInit(missingUserPassAndAPITokenValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because one auth mode is required", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("provider.apiToken or provider.username/provider.password should be set")))
		})
	})

	Context("Cluster with module configuration and both user/password and apiToken", func() {
		cfg := HookExecutionConfigInit(hasUserPassAndAPITokenValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should fail because auth modes are mutually exclusive", func() {
			Expect(cfg).ToNot(ExecuteSuccessfully())
			Expect(cfg.GoHookError).To(MatchError(ContainSubstring("provider authentication must use either apiToken or username/password")))
		})
	})

	Context("Hybrid cluster without discovery data in secret and values", func() {
		cfg := HookExecutionConfigInit(hybridValuesWithoutDiscoveryData, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should set default discovery data values", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VCDCloudProviderDiscoveryData",
  "zones": ["default"]
}
`))
		})
	})

	Context("Hybrid cluster with partial inline discovery data without defaults", func() {
		cfg := HookExecutionConfigInit(partialInlineDiscoveryValuesWithoutDefaults, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(""))
			cfg.RunHook()
		})

		It("Should keep inline fields and apply missing defaults", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VCDCloudProviderDiscoveryData",
  "zones": ["default"],
  "storageProfiles": [
    {
      "name": "module-profile",
      "isEnabled": true
    }
  ]
}
`))
		})
	})

	Context("Cluster with secret discovery data and partial inline discovery data", func() {
		cfg := HookExecutionConfigInit(mergeDiscoveryValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(notEmptyProviderClusterConfigurationStateWithRichDiscovery))
			cfg.RunHook()
		})

		It("Should merge missing discovery fields from secret and preserve existing inline ones", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VCDCloudProviderDiscoveryData",
  "sizingPolicies": ["secret-policy"],
  "internalNetworks": ["secret-network"],
  "zones": ["module-zone"],
  "storageProfiles": [
    {
      "name": "module-profile",
      "isEnabled": true
    }
  ],
  "vcdAPIVersion": "37.3",
  "vcdInstallationVersion": "10.5",
  "loadBalancer": {
    "enabled": true
  }
}
`))
		})
	})

	Context("Cluster with inline load balancer discovery data and bootstrap secret", func() {
		cfg := HookExecutionConfigInit(loadBalancerCurrentValues, `{}`)
		BeforeEach(func() {
			cfg.BindingContexts.Set(cfg.KubeStateSet(notEmptyProviderClusterConfigurationStateWithRichDiscovery))
			cfg.RunHook()
		})

		It("Should keep current inline load balancer when secret also has one", func() {
			Expect(cfg).To(ExecuteSuccessfully())
			Expect(cfg.ValuesGet("cloudProviderVcd.internal.providerDiscoveryData.loadBalancer").String()).To(MatchJSON(`
{
  "enabled": false
}
`))
		})
	})

	Context("overrideValues validation and provider patching", func() {
		strPtr := func(v string) *string { return &v }

		getPartialProviderCfg := func() v1.VCDProviderClusterConfiguration {
			return v1.VCDProviderClusterConfiguration{
				Provider: &v1.VCDProvider{
					Server:   strPtr("https://provider.example.com/api"),
					Username: strPtr("provider-user"),
					Password: strPtr("provider-password"),
				},
				Organization:           strPtr("org"),
				VirtualDataCenter:      strPtr("vdc"),
				VirtualApplicationName: strPtr("vapp"),
				MainNetwork:            strPtr("main-network"),
			}
		}

		It("Should return error when virtualDataCenter is missing", func() {
			cfg := getPartialProviderCfg()
			cfg.VirtualDataCenter = nil

			overrideProviderClusterConfig(&cfg, &v1.VCDModuleConfig{})
			err := validateProviderClusterConfig(cfg)
			Expect(err).To(MatchError("virtualDataCenter cannot be empty"))
		})

		It("Should return error when virtualApplicationName is missing", func() {
			cfg := getPartialProviderCfg()
			cfg.VirtualApplicationName = nil

			overrideProviderClusterConfig(&cfg, &v1.VCDModuleConfig{})
			err := validateProviderClusterConfig(cfg)
			Expect(err).To(MatchError("virtualApplicationName cannot be empty"))
		})

		It("Should return error when mainNetwork is missing", func() {
			cfg := getPartialProviderCfg()
			cfg.MainNetwork = nil

			overrideProviderClusterConfig(&cfg, &v1.VCDModuleConfig{})
			err := validateProviderClusterConfig(cfg)
			Expect(err).To(MatchError("mainNetwork cannot be empty"))
		})

		It("Should return error when kind exists but apiVersion is empty", func() {
			cfg := getPartialProviderCfg()
			cfg.Kind = strPtr("VCDClusterConfiguration")
			cfg.APIVersion = strPtr("")

			overrideProviderClusterConfig(&cfg, &v1.VCDModuleConfig{})
			err := validateProviderClusterConfig(cfg)
			Expect(err).To(MatchError("apiVersion cannot be empty"))
		})

		It("Should return error when apiVersion exists but kind is empty", func() {
			cfg := getPartialProviderCfg()
			cfg.APIVersion = strPtr("deckhouse.io/v1")
			cfg.Kind = strPtr("")

			overrideProviderClusterConfig(&cfg, &v1.VCDModuleConfig{})
			err := validateProviderClusterConfig(cfg)
			Expect(err).To(MatchError("kind cannot be empty"))
		})

		It("Should override provider apiToken from module config", func() {
			cfg := getPartialProviderCfg()
			cfg.Provider.APIToken = strPtr("secret-token")
			cfg.Provider.Username = nil
			cfg.Provider.Password = nil

			moduleCfg := v1.VCDModuleConfig{
				Provider: &v1.VCDProvider{
					APIToken: strPtr("module-token"),
				},
			}

			overrideProviderClusterConfig(&cfg, &moduleCfg)
			err := validateProviderClusterConfig(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Provider.APIToken).ToNot(BeNil())
			Expect(*cfg.Provider.APIToken).To(Equal("module-token"))
		})

		It("Should override provider username and password from module config", func() {
			cfg := getPartialProviderCfg()

			moduleCfg := v1.VCDModuleConfig{
				Provider: &v1.VCDProvider{
					Username: strPtr("module-user"),
					Password: strPtr("module-password"),
				},
			}

			overrideProviderClusterConfig(&cfg, &moduleCfg)
			err := validateProviderClusterConfig(cfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Provider.Username).ToNot(BeNil())
			Expect(cfg.Provider.Password).ToNot(BeNil())
			Expect(*cfg.Provider.Username).To(Equal("module-user"))
			Expect(*cfg.Provider.Password).To(Equal("module-password"))
			Expect(cfg.Provider.APIToken).To(BeNil())
		})
	})
})
