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

		// A malformed PCC cannot be exercised from here: FilterPCCSecret validates the payload
		// against candi/openapi/cluster_configuration.yaml, so an invalid document fails the
		// kubernetes binding itself ("couldn't enable kubernetes bindings: ... apply filter")
		// and the hook never runs. The filter's error paths are unit-tested instead, in
		// hooks/internal/filters_test.go.

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

		// A ModuleConfig already converted to v2. FilterModuleConfig fills SettingsV2 for it and
		// leaves SettingsV1 empty, which is what makes the v1 projection unsafe here.
		moduleConfigV2 = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: test
        folderID: test
    nodes:
      parameters:
        layout: WithNATInstance
        nodeNetworkCIDR: 84.201.160.148/31
        sshPublicKey: ssh-rsa AAAAAbbbb
    storage:
      parameters:
        excludedStorageClasses: ["network-hdd"]
    ccm:
      parameters:
        additionalExternalNetworkIDs: ["operator-net"]
`

		// The candi Secret exists but cloud-provider-discovery-data.json is empty: indistinguishable
		// from no Secret at all, and must not silence the PCC fallback.
		candiEmptySecretState = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-candi-cloud-provider-discovery-data
  namespace: d8-cloud-provider-yandex
data:
  "cloud-provider-discovery-data.json": ""
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

		// This is the shape a cluster whose infrastructure DKP does not create ends up with,
		// and the same payload openapi/values.yaml validates on every write. Every field of
		// YandexCloudDiscoveryData except the type markers and the region is `omitempty`, so
		// nothing else may appear here — a nil slice or map would serialize to null and be
		// rejected as "must be of type array/object: null". The schema side of the contract
		// is pinned in openapi/openapi-case-tests.yaml, the encoding side in
		// hooks/internal/cloud_discovery_data_test.go.
		It("writes discovery data carrying only the type markers and the region", func() {
			Expect(a).To(ExecuteSuccessfully())

			discoveryData := a.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData")
			Expect(discoveryData.Exists()).To(BeTrue())
			Expect(discoveryData.Map()).To(HaveLen(3))

			Expect(discoveryData.Get("apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(discoveryData.Get("kind").String()).To(Equal("YandexCloudDiscoveryData"))
			Expect(discoveryData.Get("region").String()).To(Equal("ru-central1"))

			for _, absent := range []string{
				"routeTableID",
				"defaultLbTargetGroupNetworkId",
				"internalNetworkIDs",
				"zones",
				"zoneToSubnetIdMap",
				"shouldAssignPublicIPAddress",
			} {
				Expect(discoveryData.Get(absent).Exists()).To(BeFalse(), "%s must be omitted, not written as an empty value or null", absent)
			}
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

			// The keys are the YandexCloudDiscoveryData json tags, which is also what
			// openapi/values.yaml declares for internal.providerDiscoveryData.

			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.defaultLbTargetGroupNetworkId").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.internalNetworkIDs").AsStringSlice()).To(Equal([]string{"test"}))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.region").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.routeTableID").String()).To(Equal("test"))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.shouldAssignPublicIPAddress").Bool()).To(BeFalse())
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.zoneToSubnetIdMap").String()).To(MatchYAML(`
ru-central1-a: test
ru-central1-b: test
ru-central1-c: test
`))
			Expect(b.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.zones").AsStringSlice()).To(Equal([]string{"ru-central1-a", "ru-central1-b", "ru-central1-c"}))
		})
	})

	// ---- Only the exporter credential Secret exists: the cluster is NOT migrated ----
	//
	// The credential_secrets binding matches the exporter Secret too, and it carries the same
	// credentials type. Counting snapshots would declare this cluster migrated and drop the
	// artifacts, while the terraform projection - which matches by name - would still read the
	// PCC. See the note in candi/terraform-modules/migration/locals.tf.
	exporterOnlyCluster := HookExecutionConfigInit(initValuesString, `{}`)
	exporterOnlyCluster.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	exporterOnlyCluster.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	exporterOnlyCluster.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("No PCC and only the exporter credential Secret — migration artifacts kept", func() {
		BeforeEach(func() {
			exporterOnlyCluster.BindingContexts.Set(exporterOnlyCluster.KubeStateSet(exporterOnlyCredentialState))
			exporterOnlyCluster.RunHook()
		})

		It("Hook should succeed and keep the migration artifacts", func() {
			Expect(exporterOnlyCluster).To(ExecuteSuccessfully())

			Expect(exporterOnlyCluster.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "d8-module-is-migrating").Exists()).To(BeTrue())
		})
	})

	// ---- PCC still present while the ModuleConfig is already v2 ----
	//
	// The hook projects the PCC only while the ModuleConfig is v1. Once it has been converted to
	// v2 the config values are the source of truth and no PCC-derived value may overwrite them:
	// FilterModuleConfig leaves SettingsV1 empty for a v2 ModuleConfig, so projecting anyway
	// would replace every section with a zero value - excludedStorageClasses would stop being
	// honoured (storage_classes.go would recreate the excluded StorageClasses) and the CCM would
	// lose additionalExternalNetworkIDs.
	stateBWithV2 := HookExecutionConfigInit(`
global:
  discovery: {}
cloudProviderYandex:
  internal: {}
  provider:
    parameters:
      cloudID: from-module-config
      folderID: from-module-config
  nodes:
    parameters:
      layout: Standard
      nodeNetworkCIDR: 10.10.0.0/16
      sshPublicKey: ssh-rsa from-module-config
  storage:
    parameters:
      excludedStorageClasses: ["network-hdd"]
  ccm:
    parameters:
      additionalExternalNetworkIDs: ["operator-net"]
`, `{}`)
	stateBWithV2.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	stateBWithV2.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	stateBWithV2.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("PCC present but the ModuleConfig is already v2 — config values win", func() {
		BeforeEach(func() {
			stateBWithV2.BindingContexts.Set(stateBWithV2.KubeStateSet(stateB + "\n---\n" + moduleConfigV2))
			stateBWithV2.RunHook()
		})

		It("leaves every settings section untouched, including the ones the PCC also carries", func() {
			Expect(stateBWithV2).To(ExecuteSuccessfully())

			Expect(stateBWithV2.ValuesGet("cloudProviderYandex.storage.parameters.excludedStorageClasses").AsStringSlice()).
				To(Equal([]string{"network-hdd"}))
			Expect(stateBWithV2.ValuesGet("cloudProviderYandex.ccm.parameters.additionalExternalNetworkIDs").AsStringSlice()).
				To(Equal([]string{"operator-net"}))

			// The PCC carries cloudID/folderID "test" and layout WithNATInstance; under a v2
			// ModuleConfig none of it may reach the values.
			Expect(stateBWithV2.ValuesGet("cloudProviderYandex.provider.parameters.cloudID").String()).To(Equal("from-module-config"))
			Expect(stateBWithV2.ValuesGet("cloudProviderYandex.provider.parameters.folderID").String()).To(Equal("from-module-config"))
			Expect(stateBWithV2.ValuesGet("cloudProviderYandex.nodes.parameters.layout").String()).To(Equal("Standard"))
			Expect(stateBWithV2.ValuesGet("cloudProviderYandex.nodes.parameters.nodeNetworkCIDR").String()).To(Equal("10.10.0.0/16"))
		})

		It("still writes the discovery data, so the workloads keep rendering", func() {
			Expect(stateBWithV2).To(ExecuteSuccessfully())

			// The PCC payload is the only discovery source here, and it must go through
			// MergeDiscoveryData: the type markers and the region have to be present.
			discoveryData := stateBWithV2.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData")
			Expect(discoveryData.Get("apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(discoveryData.Get("kind").String()).To(Equal("YandexCloudDiscoveryData"))
			Expect(discoveryData.Get("routeTableID").String()).To(Equal("test"))
		})
	})

	// ---- Migration artifacts: dropped once the new model stands on its own ----
	//
	// While d8-module-is-migrating exists, ShouldSkipNewModelValidation keeps new-model
	// validation switched off, so a migrated cluster has to lose the artifacts.
	migratedCluster := HookExecutionConfigInit(initValuesString, `{}`)
	migratedCluster.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	migratedCluster.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	migratedCluster.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("No PCC, credentials and a v2 ModuleConfig — migration artifacts deleted", func() {
		BeforeEach(func() {
			migratedCluster.BindingContexts.Set(migratedCluster.KubeStateSet(credentialSecretState + "\n---\n" + moduleConfigV2))
			migratedCluster.RunHook()
		})

		It("Hook should succeed and delete the migration artifacts", func() {
			Expect(migratedCluster).To(ExecuteSuccessfully())

			Expect(migratedCluster.KubernetesResource("Secret", "d8-cloud-provider-yandex", "d8-migration-resources").Exists()).To(BeFalse())
			Expect(migratedCluster.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "d8-module-is-migrating").Exists()).To(BeFalse())
		})
	})

	// The credential Secret alone is not enough: it is only one of the four new-model resources.
	// A cluster whose ModuleConfig is still v1 has lost the PCC but has nowhere to read its
	// configuration from yet, and dropping d8-module-is-migrating there would let
	// ShouldSkipNewModelValidation stop suppressing new-model validation while the configuration
	// is still in the old shape.
	credentialsOnlyCluster := HookExecutionConfigInit(initValuesString, `{}`)
	credentialsOnlyCluster.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	credentialsOnlyCluster.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	credentialsOnlyCluster.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("No PCC and credentials but the ModuleConfig is still v1 — artifacts kept", func() {
		BeforeEach(func() {
			credentialsOnlyCluster.BindingContexts.Set(credentialsOnlyCluster.KubeStateSet(credentialSecretState + "\n---\n" + moduleConfigV1))
			credentialsOnlyCluster.RunHook()
		})

		It("Hook should succeed and keep the migration artifacts", func() {
			Expect(credentialsOnlyCluster).To(ExecuteSuccessfully())

			Expect(credentialsOnlyCluster.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "d8-module-is-migrating").Exists()).To(BeTrue())
		})
	})

	// A cluster without PCC and without credentials is not migrated yet: the artifacts stay,
	// otherwise the hook would stop projecting the PCC while terraform still reads it.
	notMigratedCluster := HookExecutionConfigInit(initValuesString, `{}`)
	notMigratedCluster.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	notMigratedCluster.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	notMigratedCluster.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("No PCC and no credentials — migration artifacts kept", func() {
		BeforeEach(func() {
			notMigratedCluster.BindingContexts.Set(notMigratedCluster.KubeStateSet(migrationArtifactsState))
			notMigratedCluster.RunHook()
		})

		It("Hook should succeed and keep the migration artifacts", func() {
			Expect(notMigratedCluster).To(ExecuteSuccessfully())

			Expect(notMigratedCluster.KubernetesResource("ConfigMap", "d8-cloud-provider-yandex", "d8-module-is-migrating").Exists()).To(BeTrue())
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

	// ---- Context candi-empty-with-pcc: empty candi payload does not silence the PCC fallback ----
	candiEmptyWithPCC := HookExecutionConfigInit(initValuesString, `{}`)
	candiEmptyWithPCC.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
	candiEmptyWithPCC.RegisterCRD("deckhouse.io", "v1", "YandexInstanceClass", false)
	candiEmptyWithPCC.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	Context("Empty candi Secret payload plus PCC with discovery data yields values from PCC", func() {
		BeforeEach(func() {
			candiEmptyWithPCC.BindingContexts.Set(candiEmptyWithPCC.KubeStateSet(stateB + "\n---\n" + moduleConfigV1 + "\n---\n" + candiEmptySecretState))
			candiEmptyWithPCC.RunHook()
		})

		It("Discovery data values come from PCC, not an empty object", func() {
			Expect(candiEmptyWithPCC).To(ExecuteSuccessfully())

			Expect(candiEmptyWithPCC.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.defaultLbTargetGroupNetworkId").String()).To(Equal("test"))
			Expect(candiEmptyWithPCC.ValuesGet("cloudProviderYandex.internal.providerDiscoveryData.routeTableID").String()).To(Equal("test"))
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
      provision:
        - name: network-ssd-64k
          type: network-ssd
          blockSize: 64Ki
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

		It("storage and ccm sections exist, storageClass settings projected onto storage.parameters", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.storage.parameters").Exists()).To(BeTrue())
			Expect(f.ValuesGet("cloudProviderYandex.ccm.parameters").Exists()).To(BeTrue())
			Expect(f.ValuesGet("cloudProviderYandex.storage.parameters.excludedStorageClasses").String()).
				To(MatchJSON(`["network-hdd", "network-ssd-nonreplicated"]`))
			Expect(f.ValuesGet("cloudProviderYandex.storage.parameters.provisionedStorageClasses").String()).
				To(MatchJSON(`[{"name": "network-ssd-64k", "type": "network-ssd", "blockSize": "64Ki"}]`))
		})

		// The v1 path is not mirrored any more: storage_classes.go reads
		// storage.parameters, and the ModuleConfig v2 schema has no storageClass section.
		It("does not write the legacy storageClass values path", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderYandex.storageClass").Exists()).To(BeFalse())
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

const credentialSecretState = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-yandex
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: serviceAccount
  secret: '{"id": "test"}'
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-yandex
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-yandex
data: {}
`

const migrationArtifactsState = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-yandex
data: {}
`

// The NAT-instance exporter Secret carries the same credentials type as the managed one, so it
// lands in the same snapshot. On its own it does not make a cluster migrated.
const exporterOnlyCredentialState = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials-exporter
  namespace: d8-cloud-provider-yandex
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: apiToken
  secret: exporter-key-abc
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-yandex
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-yandex
data: {}
`
