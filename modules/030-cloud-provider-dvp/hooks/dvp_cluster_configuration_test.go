/*
Copyright 2026 Flant JSC

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

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: dvp_cluster_configuration ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
`
	)

	stateACloudDiscoveryData := `
{
   "apiVersion": "deckhouse.io/v1",
   "kind": "DVPCloudDiscoveryData",
   "zones": ["default"]
}
`
	stateAClusterConfiguration1 := `
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
masterNodeGroup:
  instanceClass:
    etcdDisk:
      size: 15Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    rootDisk:
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
      size: 50Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
    virtualMachine:
      virtualMachineClassName: superbe-class
      bootloader: EFI
      cpu:
        coreFraction: 100%
        cores: 4
      liveMigrationPolicy: PreferForced
      runPolicy: AlwaysOnUnlessStoppedManually
      ipAddresses:
        - Auto
      memory:
        size: 8Gi
  replicas: 3
provider:
  kubeconfigDataBase64: YXBpVmV=
  namespace: cloud-provider01
sshPublicKey: ssh-rsa AAAAB3N
region: ru-msk-1
zones:
- default
`

	notEmptyPCCState := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)), base64.StdEncoding.EncodeToString([]byte(stateACloudDiscoveryData)))

	// ---- State A: no PCC ----
	Context("State A: no PCC (new v2 cluster)", func() {
		f := HookExecutionConfigInit(emptyValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should execute successfully and not create migration artifacts", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())

			Expect(f.ValuesGet("cloudProviderDvp.internal.providerClusterConfiguration").Exists()).To(BeFalse())

			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.kind").String()).To(Equal("DVPCloudDiscoveryData"))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["default"]`))
		})
	})

	// ---- State A with stale migration artifacts ----
	Context("State A: no PCC but stale migration artifacts exist", func() {
		f := HookExecutionConfigInit(emptyValues, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should clean up migration artifacts", func() {
			Expect(f).To(ExecuteSuccessfully())

			migrationSecret := f.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())

			migrationCM := f.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())
		})
	})

	// ---- State B: PCC present, new resources absent ----
	Context("State B: PCC present, new resources not yet applied", func() {
		b := HookExecutionConfigInit(emptyValues, `{}`)
		b.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		b.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		b.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			b.KubeStateSet(notEmptyPCCState)
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("should fill values from PCC and create migration artifacts", func() {
			Expect(b).To(ExecuteSuccessfully())

			// Root values should be set from PCC.
			Expect(b.ValuesGet("cloudProviderDvp.provider.parameters.namespace").String()).To(Equal("cloud-provider01"))
			Expect(b.ValuesGet("cloudProviderDvp.nodes.parameters.layout").String()).To(Equal("Standard"))
			Expect(b.ValuesGet("cloudProviderDvp.nodes.parameters.sshPublicKey").String()).To(Equal("ssh-rsa AAAAB3N"))
			Expect(b.ValuesGet("cloudProviderDvp.nodes.parameters.region").String()).To(Equal("ru-msk-1"))

			// internal.providerClusterConfiguration should NOT be set (templates no longer need it).
			Expect(b.ValuesGet("cloudProviderDvp.internal.providerClusterConfiguration").Exists()).To(BeFalse())

			// Synthetic d8-credentials should be injected from PCC kubeconfigDataBase64.
			Expect(b.ValuesGet("cloudProviderDvp.internal.credentialSecrets.d8-credentials.authScheme").String()).To(Equal("kubeconfig"))
			Expect(b.ValuesGet("cloudProviderDvp.internal.credentialSecrets.d8-credentials.secret").String()).To(Equal("YXBpVmV="))

			Expect(b.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData").String()).To(MatchJSON(stateACloudDiscoveryData))

			// Migration resources secret should be created.
			migrationSecret := b.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())
			resourcesManifest, err := base64.StdEncoding.DecodeString(migrationSecret.Field(`data.resources\.yaml`).String())
			Expect(err).ToNot(HaveOccurred())
			Expect(string(resourcesManifest)).To(MatchYAML(`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  enabled: true
  version: 2
  settings:
    provider:
      parameters:
        namespace: cloud-provider01
    storage:
      enabled: true
      parameters: {}
    nodes:
      enabled: true
      parameters:
        layout: Standard
        sshPublicKey: ssh-rsa AAAAB3N
        region: ru-msk-1
        zones:
        - default
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-dvp-credentials
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: S3ViZWNvbmZpZw==
  secret: YXBpVmV=
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
spec:
  etcdDisk:
    size: 15Gi
    storageClass: ceph-pool-r2-csi-rbd-immediate
  rootDisk:
    image:
      kind: ClusterVirtualImage
      name: ubuntu-2204
    size: 50Gi
    storageClass: ceph-pool-r2-csi-rbd-immediate
  virtualMachine:
    virtualMachineClassName: superbe-class
    bootloader: EFI
    cpu:
      coreFraction: 100%
      cores: 4
    liveMigrationPolicy: PreferForced
    runPolicy: AlwaysOnUnlessStoppedManually
    ipAddresses:
    - Auto
    memory:
      size: 8Gi
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
spec:
  nodeType: CloudPermanent
  cloudInstances:
    zones:
    - default
    minPerZone: 3
    maxPerZone: 3
    classReference:
      kind: DVPInstanceClass
      name: master-dvp
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
`))

			// Migration configmap should be created.
			migrationCM := b.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeTrue())
			Expect(migrationCM.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
			Expect(migrationCM.Field("metadata.labels.module").String()).To(Equal("cloud-provider-dvp"))

			// Resources should NOT be created directly by the hook.
			moduleConfig := b.KubernetesGlobalResource("ModuleConfig", "cloud-provider-dvp")
			Expect(moduleConfig.Exists()).To(BeFalse())
		})
	})

	// ---- State B with real credential already present (from credentials.go at Order 19) ----
	Context("State B: PCC present, real d8-credentials Secret already populated by credentials.go", func() {
		// Simulate that credentials.go (Order 19) has already run and written the real secret
		// into values by pre-seeding internal.credentialSecrets.d8-credentials.
		valuesWithRealCred := `
global:
  discovery: {}
cloudProviderDvp:
  internal:
    credentialSecrets:
      d8-credentials:
        authScheme: kubeconfig
        secret: cmVhbC1rdWJlY29uZmlnLWRhdGE=
`
		bReal := HookExecutionConfigInit(valuesWithRealCred, `{}`)
		bReal.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		bReal.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		bReal.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			bReal.KubeStateSet(notEmptyPCCState)
			bReal.BindingContexts.Set(bReal.GenerateBeforeHelmContext())
			bReal.RunHook()
		})

		It("should NOT overwrite real d8-credentials with PCC-synthetic value", func() {
			Expect(bReal).To(ExecuteSuccessfully())

			// Real credential must be preserved — PCC-synthetic must NOT overwrite it.
			Expect(bReal.ValuesGet("cloudProviderDvp.internal.credentialSecrets.d8-credentials.authScheme").String()).To(Equal("kubeconfig"))
			Expect(bReal.ValuesGet("cloudProviderDvp.internal.credentialSecrets.d8-credentials.secret").String()).To(Equal("cmVhbC1rdWJlY29uZmlnLWRhdGE="))
		})
	})

	// ---- State C: PCC present, all new resources fully applied ----
	Context("State C: PCC present, all new resources applied (migration complete)", func() {
		c := HookExecutionConfigInit(emptyValues, `{}`)
		c.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		c.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		c.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			stateCResources := fmt.Sprintf(`
%s
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-dvp
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        namespace: cloud-provider01
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: ssh-rsa AAAAB3N
    storage:
      parameters: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: S3ViZWNvbmZpZw==
  secret: YXBpVmV=
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master-dvp
spec: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
`, notEmptyPCCState)
			c.KubeStateSet(stateCResources)
			c.BindingContexts.Set(c.GenerateBeforeHelmContext())
			c.RunHook()
		})

		It("should ignore PCC for root values and clean up migration artifacts", func() {
			Expect(c).To(ExecuteSuccessfully())

			// Migration artifacts should be cleaned up.
			migrationCM := c.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())

			migrationSecret := c.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())

			// internal.providerClusterConfiguration should NOT be set (templates no longer need it).
			Expect(c.ValuesGet("cloudProviderDvp.internal.providerClusterConfiguration").Exists()).To(BeFalse())

			// Discovery data should be populated from PCC secret.
			Expect(c.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(c.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["default"]`))
		})
	})
})
