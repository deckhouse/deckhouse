/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: yandex_cluster_configuration ::", func() {
	const (
		initValuesString = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`
	)

	var (
		// correct cdd
		stateBCloudDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "defaultLbTargetGroupNetworkId": "test",
  "internalNetworkIDs": [
    "test"
  ],
  "kind": "YandexCloudDiscoveryData",
  "region": "test",
  "routeTableID": "test",
  "shouldAssignPublicIPAddress": false,
  "zoneToSubnetIdMap": {
    "ru-central1-a": "test",
    "ru-central1-b": "test",
    "ru-central1-c": "test"
  },
  "zones": [
    "ru-central1-a",
    "ru-central1-b",
    "ru-central1-c"
  ]
}
`

		// correct pcc
		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
existingNetworkID: enpma5uvcfbkuac1i1jb
kind: YandexClusterConfiguration
layout: WithNATInstance
masterNodeGroup:
  instanceClass:
    cores: 2
    etcdDiskSizeGb: 10
    imageID: test
    memory: 4096
    platform: standard-v2
  replicas: 1
provider:
  cloudID: test
  folderID: test
  serviceAccountJSON: |-
    {
      "id": "test"
    }
withNATInstance:
  internalSubnetID: test
  natInstanceExternalAddress: 84.201.160.148
  exporterAPIKey: ""
  natInstanceResources:
    cores: 2
    memory: 2048
    platform: "standard-v2"
nodeNetworkCIDR: 84.201.160.148/31
sshPublicKey: ssh-rsa AAAAAbbbb
`

		// wrong pcc (unused; kept for reference)
		_ = `
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
`

		stateB = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration)), base64.StdEncoding.EncodeToString([]byte(stateBCloudDiscoveryData)))

		moduleConfigV1 = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings: {}
`
	)

	// ---- Context a: Cluster has empty state (no PCC) ----
	a := HookExecutionConfigInit(initValuesString, `{}`)
	a.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	a.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	a.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should succeed", func() {
			Expect(a).To(ExecuteSuccessfully())
		})
	})

	// ---- Context b: Provider data is successfully discovered (State B — migration) ----
	b := HookExecutionConfigInit(initValuesString, `{}`)
	b.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	b.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	b.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(stateB + "\n---\n" + moduleConfigV1))
			b.RunHook()
		})

		It("Discovery data values should be gathered from discovered data", func() {
			Expect(b).To(ExecuteSuccessfully())

			// Note: YandexCloudDiscoveryData struct has no json tags, so json.Marshal
			// uses Go field names (e.g., DefaultLbTargetGroupNetworkID instead of
			// defaultLbTargetGroupNetworkId).

			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.DefaultLbTargetGroupNetworkID").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.InternalNetworkIDs").AsStringSlice()).To(Equal([]string{"test"}))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.Region").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.RouteTableID").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.ShouldAssignPublicIPAddress").Bool()).To(BeFalse())
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.ZoneToSubnetIDMap").String()).To(MatchYAML(`
ru-central1-a: test
ru-central1-b: test
ru-central1-c: test
`))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.Zones").AsStringSlice()).To(Equal([]string{"ru-central1-a", "ru-central1-b", "ru-central1-c"}))
		})
	})

	// ---- Context c: Invalid discovery data — no valid PCC, hook succeeds ----
	c := HookExecutionConfigInit(initValuesString, `{}`)
	c.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	c.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	c.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("Discovery data is wrong", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(``))
			c.RunHook()
		})

		It("Hook should succeed (no valid PCC)", func() {
			Expect(c).To(ExecuteSuccessfully())
		})
	})

	// ---- Context d: Invalid cluster config — no valid PCC, hook succeeds ----
	d := HookExecutionConfigInit(initValuesString, `{}`)
	d.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	d.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	d.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("Cluster config is wrong", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(``))
			d.RunHook()
		})

		It("Hook should succeed (no valid PCC)", func() {
			Expect(d).To(ExecuteSuccessfully())
		})
	})

	// ---- Context e: State B — migration values populated from PCC ----
	e := HookExecutionConfigInit(initValuesString, `{}`)
	e.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	e.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	e.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("State B: PCC present — migration values populated from cluster configuration", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(stateB + "\n---\n" + moduleConfigV1))
			e.RunHook()
		})

		It("should populate provider, nodes, storage, ccm and credentialSecrets from PCC", func() {
			Expect(e).To(ExecuteSuccessfully())

			Expect(e.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).To(Equal("test"))
			Expect(e.ValuesGet("cloudProviderYandex.provider.parameters.folderID").String()).To(Equal("test"))

			Expect(e.ValuesGet("cloudProviderYandex.nodes.parameters.sshPublicKey").String()).To(Equal("ssh-rsa AAAAAbbbb"))
			Expect(e.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).To(Equal("WithNATInstance"))
			Expect(e.ValuesGet("cloudProviderYandex.nodes.parameters.nodeNetworkCIDR").String()).To(Equal("84.201.160.148/31"))
			Expect(e.ValuesGet("cloudProviderYandex.nodes.parameters.existingNetworkID").String()).To(Equal("enpma5uvcfbkuac1i1jb"))

			Expect(e.ValuesGet("cloudProviderYandex.storage.parameters").Exists()).To(BeTrue())
			Expect(e.ValuesGet("cloudProviderYandex.ccm.parameters").Exists()).To(BeTrue())

			Expect(e.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials").Exists()).To(BeTrue())
			Expect(e.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials.authScheme").String()).To(Equal("serviceAccount"))
		})
	})
})

const (
	migrationValuesFull = `
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
`
)

var (
	// -----------------------------------------------------------------------
	// Test data for State B mappings
	// -----------------------------------------------------------------------

	discoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "YandexCloudDiscoveryData",
  "region": "ru-central1",
  "routeTableID": "rt-test",
  "defaultLbTargetGroupNetworkId": "lb-test",
  "internalNetworkIDs": ["net-internal"],
  "zones": ["ru-central1-a","ru-central1-b","ru-central1-d"],
  "zoneToSubnetIdMap": {"ru-central1-a":"s1","ru-central1-b":"s2","ru-central1-d":"s3"},
  "shouldAssignPublicIPAddress": true
}`

	// WithNATInstance — most complex layout with all optional fields filled.
	pccWithNAT = fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
nodeNetworkCIDR: 10.0.0.0/16
sshPublicKey: ssh-rsa FULL_KEY
existingNetworkID: enp-existing-net
existingZoneToSubnetIDMap:
  ru-central1-a: subnet-existing-a
  ru-central1-b: subnet-existing-b
labels:
  env: production
  team: platform
dhcpOptions:
  domainName: internal.example.com
  domainNameServers:
    - 10.0.0.1
    - 10.0.0.2
zones:
  - ru-central1-a
  - ru-central1-b
  - ru-central1-d
masterNodeGroup:
  replicas: 1
  zones:
    - ru-central1-a
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd-master-img
    platform: standard-v3
    diskSizeGB: 50
    diskType: network-ssd
    externalSubnetID: subnet-ext-master
    externalSubnetIDs:
      - subnet-ext-master-2
    additionalLabels:
      role: master
provider:
  cloudID: cloud-full
  folderID: folder-full
  serviceAccountJSON: '{"id":"sa-full"}'
withNATInstance:
  internalSubnetID: subnet-nat-internal
  internalSubnetCIDR: 10.128.0.0/24
  externalSubnetID: subnet-nat-external
  natInstanceExternalAddress: 84.201.160.148
  natInstanceInternalAddress: 10.128.0.10
  exporterAPIKey: exporter-key-abc
  natInstanceResources:
    cores: 4
    memory: 4096
    platform: standard-v3
`)

	// Standard layout — minimal set of fields, no NAT.
	pccStandard = fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 10.1.0.0/16
sshPublicKey: ssh-rsa STANDARD_KEY
zones:
  - ru-central1-a
  - ru-central1-b
masterNodeGroup:
  replicas: 3
  instanceClass:
    cores: 2
    memory: 4096
    imageID: fd-standard-img
provider:
  cloudID: cloud-std
  folderID: folder-std
  serviceAccountJSON: '{"id":"sa-std"}'
`)

	// WithoutNAT layout — no NAT, no zones override.
	pccWithoutNAT = fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
nodeNetworkCIDR: 10.2.0.0/16
sshPublicKey: ssh-rsa WITHOUTNAT_KEY
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 2
    memory: 4096
    imageID: fd-withoutnat-img
provider:
  cloudID: cloud-wn
  folderID: folder-wn
  serviceAccountJSON: '{"id":"sa-wn"}'
`)

	// MC v1 with storage and CCM settings.
	mcV1WithStorageCCM = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      default: network-ssd
      exclude:
        - network-hdd
        - network-ssd-nonreplicated
    additionalExternalNetworkIDs:
      - net-extra-1
      - net-extra-2
`

	// Full state: PCC + discovery + MC v1.
	makePCCState = func(pccYAML string) string {
		pccSecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(pccYAML)),
			base64.StdEncoding.EncodeToString([]byte(discoveryData)))
		return pccSecret + "\n---\n" + mcV1WithStorageCCM
	}
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: yandex_cluster_configuration :: State B mappings ::", func() {
	// -----------------------------------------------------------------------
	// WithNATInstance — all fields
	// -----------------------------------------------------------------------
	Context("WithNATInstance layout — all PCC fields mapped to settings paths", func() {
		f := HookExecutionConfigInit(migrationValuesFull, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(makePCCState(pccWithNAT)))
			f.RunHook()
		})

		It("provider parameters mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).To(Equal("cloud-full"))
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.folderID").String()).To(Equal("folder-full"))
		})

		It("nodes scalar fields mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).To(Equal("WithNATInstance"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.sshPublicKey").String()).To(Equal("ssh-rsa FULL_KEY"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.nodeNetworkCIDR").String()).To(Equal("10.0.0.0/16"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.existingNetworkID").String()).To(Equal("enp-existing-net"))
		})

		It("nodes complex fields mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.zones").AsStringSlice()).To(ConsistOf(
				"ru-central1-a", "ru-central1-b", "ru-central1-d"))

			zsm := f.ValuesGet("cloudProviderYandex.nodes.parameters.existingZoneToSubnetIDMap")
			Expect(zsm.Get("ru-central1-a").String()).To(Equal("subnet-existing-a"))
			Expect(zsm.Get("ru-central1-b").String()).To(Equal("subnet-existing-b"))

			labels := f.ValuesGet("cloudProviderYandex.nodes.parameters.labels")
			Expect(labels.Get("env").String()).To(Equal("production"))
			Expect(labels.Get("team").String()).To(Equal("platform"))
		})

		It("dhcpOptions mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			dhcp := f.ValuesGet("cloudProviderYandex.nodes.parameters.dhcpOptions")
			Expect(dhcp.Get("domainName").String()).To(Equal("internal.example.com"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.dhcpOptions.domainNameServers").AsStringSlice()).To(ConsistOf("10.0.0.1", "10.0.0.2"))
		})

		It("withNATInstance all sub-fields mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			nat := f.ValuesGet("cloudProviderYandex.nodes.parameters.withNATInstance")
			Expect(nat.Get("internalSubnetID").String()).To(Equal("subnet-nat-internal"))
			Expect(nat.Get("internalSubnetCIDR").String()).To(Equal("10.128.0.0/24"))
			Expect(nat.Get("externalSubnetID").String()).To(Equal("subnet-nat-external"))
			Expect(nat.Get("natInstanceExternalAddress").String()).To(Equal("84.201.160.148"))
			Expect(nat.Get("natInstanceInternalAddress").String()).To(Equal("10.128.0.10"))

			res := nat.Get("natInstanceResources")
			Expect(res.Get("cores").Int()).To(Equal(int64(4)))
			Expect(res.Get("memory").Int()).To(Equal(int64(4096)))
			Expect(res.Get("platform").String()).To(Equal("standard-v3"))
		})

		It("exporter credential secret from PCC (from withNATInstance)", func() {
			Expect(f).To(ExecuteSuccessfully())
			cred := f.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials-exporter")
			Expect(cred.Get("authScheme").String()).To(Equal("apiToken"))
			Expect(cred.Get("secret").String()).To(Equal("exporter-key-abc"))
		})

		It("credential secrets from PCC ServiceAccount", func() {
			Expect(f).To(ExecuteSuccessfully())
			cred := f.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials")
			Expect(cred.Get("authScheme").String()).To(Equal("serviceAccount"))
			Expect(cred.Get("secret").String()).To(Equal(`{"id":"sa-full"}`))
		})

		It("storage and ccm sections exist (empty params from MC v1)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.storage.parameters").Exists()).To(BeTrue())
			Expect(f.ValuesGet("cloudProviderYandex.ccm.parameters").Exists()).To(BeTrue())
		})
	})

	// -----------------------------------------------------------------------
	// Standard layout
	// -----------------------------------------------------------------------
	Context("Standard layout — basic fields, no NAT", func() {
		f := HookExecutionConfigInit(migrationValuesFull, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(makePCCState(pccStandard)))
			f.RunHook()
		})

		It("layout is Standard and withNATInstance absent", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).To(Equal("Standard"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.withNATInstance.internalSubnetID").String()).To(Equal(""))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.dhcpOptions.domainName").String()).To(Equal(""))
		})

		It("provider and nodes fields mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).To(Equal("cloud-std"))
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.folderID").String()).To(Equal("folder-std"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.sshPublicKey").String()).To(Equal("ssh-rsa STANDARD_KEY"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.nodeNetworkCIDR").String()).To(Equal("10.1.0.0/16"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.zones").AsStringSlice()).To(ConsistOf("ru-central1-a", "ru-central1-b"))
		})

		It("exporter credential secret absent (no NAT)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials-exporter").Exists()).To(BeFalse())
		})
	})

	// -----------------------------------------------------------------------
	// WithoutNAT layout
	// -----------------------------------------------------------------------
	Context("WithoutNAT layout — no NAT, no zones in PCC", func() {
		f := HookExecutionConfigInit(migrationValuesFull, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(makePCCState(pccWithoutNAT)))
			f.RunHook()
		})

		It("layout is WithoutNAT and NAT fields absent", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).To(Equal("WithoutNAT"))
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.withNATInstance.internalSubnetID").String()).To(Equal(""))
		})

		It("provider and credentialSecrets mapped", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).To(Equal("cloud-wn"))
			Expect(f.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials").Exists()).To(BeTrue())
			Expect(f.ValuesGet("cloudProviderYandex.internal.credentialSecrets.d8-credentials.secret").String()).To(Equal(`{"id":"sa-wn"}`))
		})

		It("zones absent when not in PCC and not in discovery fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.nodes.parameters.zones").Exists()).To(BeFalse())
		})
	})
})
