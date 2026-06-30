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

	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: dvp_cluster_configuration ::", func() {
	const (
		emptyValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
  nodes: {}
  provider: {}
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
  name: d8-provider-cluster-configuration
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
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
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

		It("should fill values from PCC without creating migration artifacts (artifacts created by OnAfterHelm hook)", func() {
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

			// Migration resources secret and configmap are created by the OnAfterHelm hook (dvp_migration_resources.go),
			// NOT by this OnBeforeHelm hook. The namespace doesn't exist yet at OnBeforeHelm time.
			migrationSecret := b.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())
			migrationCM := b.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())

			// Resources should NOT be created directly by the hook.
			moduleConfig := b.KubernetesGlobalResource("ModuleConfig", "cloud-provider-dvp")
			Expect(moduleConfig.Exists()).To(BeFalse())
		})
	})

	// ---- setDefaultZones: direct unit test for the live-hook default-zone fallback ----
	Context("setDefaultZones default-zone fallback", func() {
		strPtr := func(s string) *string { return &s }
		zonesPtr := func(z []string) *[]string { return &z }

		It("injects [default] when PCC has object metadata and no zones", func() {
			p := v1.DvpProviderClusterConfiguration{
				APIVersion: strPtr("deckhouse.io/v1"),
				Kind:       strPtr("DVPClusterConfiguration"),
			}
			setDefaultZones(&p)
			Expect(p.Zones).NotTo(BeNil(), "zones must be set to the synthetic default")
			Expect(*p.Zones).To(Equal([]string{"default"}))
		})

		It("preserves existing zones when PCC already has zones", func() {
			p := v1.DvpProviderClusterConfiguration{
				APIVersion: strPtr("deckhouse.io/v1"),
				Kind:       strPtr("DVPClusterConfiguration"),
				Zones:      zonesPtr([]string{"ru-msk-1", "ru-msk-2"}),
			}
			setDefaultZones(&p)
			Expect(*p.Zones).To(Equal([]string{"ru-msk-1", "ru-msk-2"}),
				"existing zones must not be overwritten")
		})

		It("does not inject default when PCC has no object metadata", func() {
			p := v1.DvpProviderClusterConfiguration{}
			setDefaultZones(&p)
			Expect(p.Zones).To(BeNil(), "default zone must only apply to cluster-stored objects")
		})
	})

	// ---- State B with the nodes path absent in values (legacy cluster, MC v2 not applied) ----
	Context("State B: PCC present, cloudProviderDvp.nodes absent in values", func() {
		// Legacy clusters have no cloud-provider-dvp ModuleConfig, and the nodes object
		// has no own default (only nodes.disabled does), so cloudProviderDvp.nodes is
		// absent. mapPCCtoRootValues must create the whole nodes object instead of
		// patching nodes.parameters, otherwise a JSON-patch "missing path" error blocks
		// the main queue.
		legacyValues := `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
  provider: {}
`
		bNoNodes := HookExecutionConfigInit(legacyValues, `{}`)
		bNoNodes.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		bNoNodes.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		bNoNodes.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			bNoNodes.KubeStateSet(notEmptyPCCState)
			bNoNodes.BindingContexts.Set(bNoNodes.GenerateBeforeHelmContext())
			bNoNodes.RunHook()
		})

		It("should create the nodes object and fill parameters from PCC", func() {
			Expect(bNoNodes).To(ExecuteSuccessfully())

			Expect(bNoNodes.ValuesGet("cloudProviderDvp.nodes.parameters.layout").String()).To(Equal("Standard"))
			Expect(bNoNodes.ValuesGet("cloudProviderDvp.nodes.parameters.sshPublicKey").String()).To(Equal("ssh-rsa AAAAB3N"))
			Expect(bNoNodes.ValuesGet("cloudProviderDvp.nodes.parameters.region").String()).To(Equal("ru-msk-1"))
		})
	})

	// ---- State B with real credential already present (from credentials.go at Order 19) ----
	Context("State B: PCC present, real d8-credentials Secret already populated by credentials.go", func() {
		// Simulate that credentials.go (Order 19) has already run and written the real secret
		// into values by pre-seeding internal.credentialSecrets.d8-credentials.
		valuesWithRealCred := fmt.Sprintf(`
global:
  discovery: {}
cloudProviderDvp:
  internal:
    credentialSecrets:
      d8-credentials:
        authScheme: kubeconfig
        secret: %s
  nodes: {}
  provider: {}
`, base64.StdEncoding.EncodeToString([]byte("real-kubeconfig-data")))
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

			// Real credential must be preserved - PCC-synthetic must NOT overwrite it.
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
  authScheme: %s
  secret: %s
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
  name: master-fc613b4dfd67
spec: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-dvp
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
`, notEmptyPCCState, base64.StdEncoding.EncodeToString([]byte("kubeconfig")), base64.StdEncoding.EncodeToString([]byte("apiVe")))
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

	// ---- State C triggered by NodeGroup/IC event (ExecuteHookOnEvents=true path) ----
	// Validates that the hook correctly cleans up migration artifacts when triggered
	// by a NodeGroup Added event (simulated via KubeStateSet) rather than OnBeforeHelm.
	// In production this fires as a standalone ModuleHookRun via OnKubernetesEvent binding.
	Context("State C: migration complete detected via NodeGroup/DVPInstanceClass Added event", func() {
		cEvent := HookExecutionConfigInit(emptyValues, `{}`)
		cEvent.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		cEvent.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		cEvent.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			// All migration resources applied. Migration artifacts still exist (not yet cleaned).
			stateCEventResources := fmt.Sprintf(`
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
  authScheme: %s
  secret: %s
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
  name: master-fc613b4dfd67
spec: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-dvp
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
`, notEmptyPCCState, base64.StdEncoding.EncodeToString([]byte("kubeconfig")), base64.StdEncoding.EncodeToString([]byte("apiVe")))
			// KubeStateSet populates snapshots (simulates the NodeGroup Added event path).
			// GenerateBeforeHelmContext exercises the same handler function with all snapshots loaded.
			cEvent.KubeStateSet(stateCEventResources)
			cEvent.BindingContexts.Set(cEvent.GenerateBeforeHelmContext())
			cEvent.RunHook()
		})

		It("should delete migration artifacts when all target resources are present", func() {
			Expect(cEvent).To(ExecuteSuccessfully())

			migrationSecret := cEvent.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeFalse())

			migrationCM := cEvent.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeFalse())
		})
	})

	// ---- Partial migration: some resources applied, migration NOT complete ----
	// Validates that the hook stays in State B when only part of the target resources exist.
	// Specifically: NodeGroup "master" exists but generated DVPInstanceClass is missing.
	// The hook must NOT delete migration artifacts in this case.
	Context("State B partial: NodeGroup applied but DVPInstanceClass missing (migration incomplete)", func() {
		bPartial := HookExecutionConfigInit(emptyValues, `{}`)
		bPartial.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		bPartial.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		bPartial.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			// NodeGroup master exists but generated DVPInstanceClass is absent.
			// ModuleConfig v2 and d8-credentials also absent (migration resources not fully applied).
			bPartialResources := fmt.Sprintf(`
%s
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-dvp
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
`, notEmptyPCCState)
			bPartial.KubeStateSet(bPartialResources)
			bPartial.BindingContexts.Set(bPartial.GenerateBeforeHelmContext())
			bPartial.RunHook()
		})

		It("should NOT delete migration artifacts and should remain in State B", func() {
			Expect(bPartial).To(ExecuteSuccessfully())

			// Migration artifacts must still exist - migration is not complete.
			migrationSecret := bPartial.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			migrationCM := bPartial.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeTrue())

			// Values should still be populated from PCC (State B behaviour).
			Expect(bPartial.ValuesGet("cloudProviderDvp.provider.parameters.namespace").String()).To(Equal("cloud-provider01"))
		})
	})

	// ---- Partial migration: ModuleConfig v2 applied but d8-credentials missing ----
	Context("State B partial: ModuleConfig v2 applied but d8-credentials Secret missing", func() {
		bPartialCred := HookExecutionConfigInit(emptyValues, `{}`)
		bPartialCred.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		bPartialCred.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		bPartialCred.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			// ModuleConfig v2 and NodeGroup/IC present, but d8-credentials Secret absent.
			bPartialCredResources := fmt.Sprintf(`
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
  name: master-fc613b4dfd67
spec: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-migration-resources
  namespace: d8-cloud-provider-dvp
type: Opaque
data: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
`, notEmptyPCCState)
			bPartialCred.KubeStateSet(bPartialCredResources)
			bPartialCred.BindingContexts.Set(bPartialCred.GenerateBeforeHelmContext())
			bPartialCred.RunHook()
		})

		It("should NOT delete migration artifacts when d8-credentials is missing", func() {
			Expect(bPartialCred).To(ExecuteSuccessfully())

			migrationSecret := bPartialCred.KubernetesResource("Secret", "d8-cloud-provider-dvp", "d8-migration-resources")
			Expect(migrationSecret.Exists()).To(BeTrue())

			migrationCM := bPartialCred.KubernetesResource("ConfigMap", "d8-cloud-provider-dvp", "d8-module-is-migrating")
			Expect(migrationCM.Exists()).To(BeTrue())
		})
	})

	// ===========================================================================
	// Discovery data source combinations
	// ===========================================================================
	// Four scenarios for the discovery data secret sources:
	//   1. Only PCC secret (d8-provider-cluster-configuration) — legacy path
	//   2. Only candi secret (d8-candi-cloud-provider-discovery-data) — new path, no PCC
	//   3. Both secrets present — candi takes priority
	//   4. Neither secret — hybrid install (no terraform-managed VMs), no blocking
	// ===========================================================================

	var (
		// zones is the only reliable differentiator: both discovery sources have the field
		// in the schema (additionalProperties: false, layout is not in DVP schema).
		candiDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DVPCloudDiscoveryData",
  "zones": ["candi-zone"]
}
`
		pccOnlyDiscoveryData = `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DVPCloudDiscoveryData",
  "zones": ["pcc-zone"]
}
`
	)

	candiSecretState := func(discoveryJSON string) string {
		return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-candi-cloud-provider-discovery-data
  namespace: d8-cloud-provider-dvp
data:
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryJSON)))
	}

	pccStateWithDiscovery := func(discoveryJSON string) string {
		return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
  "cloud-provider-discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)), base64.StdEncoding.EncodeToString([]byte(discoveryJSON)))
	}

	// ---- Discovery source 1: only PCC secret (no candi) ----
	Context("Discovery: only PCC secret present (no candi secret) — State B", func() {
		dPCC := HookExecutionConfigInit(emptyValues, `{}`)
		dPCC.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		dPCC.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		dPCC.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			dPCC.KubeStateSet(pccStateWithDiscovery(pccOnlyDiscoveryData))
			dPCC.BindingContexts.Set(dPCC.GenerateBeforeHelmContext())
			dPCC.RunHook()
		})

		It("should use discovery data from PCC secret as fallback", func() {
			Expect(dPCC).To(ExecuteSuccessfully())
			Expect(dPCC.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["pcc-zone"]`))
		})
	})

	// ---- Discovery source 2: only candi secret (no PCC) — State A ----
	Context("Discovery: only candi secret present, no PCC — State A", func() {
		dCandi := HookExecutionConfigInit(emptyValues, `{}`)
		dCandi.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		dCandi.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		dCandi.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			dCandi.KubeStateSet(candiSecretState(candiDiscoveryData))
			dCandi.BindingContexts.Set(dCandi.GenerateBeforeHelmContext())
			dCandi.RunHook()
		})

		It("should use discovery data from candi secret", func() {
			Expect(dCandi).To(ExecuteSuccessfully())
			Expect(dCandi.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["candi-zone"]`))
		})
	})

	// ---- Discovery source 3: both secrets present — candi wins ----
	Context("Discovery: both candi and PCC secrets present — candi takes priority", func() {
		dBoth := HookExecutionConfigInit(emptyValues, `{}`)
		dBoth.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		dBoth.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		dBoth.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			bothState := pccStateWithDiscovery(pccOnlyDiscoveryData) + "\n---\n" + candiSecretState(candiDiscoveryData)
			dBoth.KubeStateSet(bothState)
			dBoth.BindingContexts.Set(dBoth.GenerateBeforeHelmContext())
			dBoth.RunHook()
		})

		It("should use candi discovery data, ignoring PCC discovery data", func() {
			Expect(dBoth).To(ExecuteSuccessfully())
			// candi-zone must win over pcc-zone
			Expect(dBoth.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["candi-zone"]`))
		})
	})

	// ---- Discovery source 4: neither secret present — hybrid install ----
	// Hybrid installations have no terraform-managed VMs; neither PCC nor candi
	// discovery secret exists. The hook must succeed with defaults and not block.
	Context("Discovery: neither PCC nor candi secret present — hybrid install (State A)", func() {
		dNone := HookExecutionConfigInit(emptyValues, `{}`)
		dNone.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		dNone.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		dNone.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			dNone.KubeStateSet(``)
			dNone.BindingContexts.Set(dNone.GenerateBeforeHelmContext())
			dNone.RunHook()
		})

		It("should succeed with default discovery data and not block the queue", func() {
			Expect(dNone).To(ExecuteSuccessfully())
			// Defaults must be applied: apiVersion, kind, zones.
			Expect(dNone.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(dNone.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.kind").String()).To(Equal("DVPCloudDiscoveryData"))
			Expect(dNone.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["default"]`))
		})
	})

	// ---- Discovery source 4b: only PCC present but without discovery data key ----
	// PCC exists (State B migration), but cloud-provider-discovery-data.json key is absent.
	// Hybrid migration case: PCC has cluster config but no terraform discovery output yet.
	Context("Discovery: PCC present but without discovery data key — hybrid migration", func() {
		dPCCNoDiscovery := HookExecutionConfigInit(emptyValues, `{}`)
		dPCCNoDiscovery.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		dPCCNoDiscovery.RegisterCRD("deckhouse.io", "v1alpha1", "DVPInstanceClass", false)
		dPCCNoDiscovery.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

		BeforeEach(func() {
			// PCC secret without the discovery data key
			pccNoDiscovery := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
data:
  "cloud-provider-cluster-configuration.yaml": %s
`, base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration1)))
			dPCCNoDiscovery.KubeStateSet(pccNoDiscovery)
			dPCCNoDiscovery.BindingContexts.Set(dPCCNoDiscovery.GenerateBeforeHelmContext())
			dPCCNoDiscovery.RunHook()
		})

		It("should succeed with defaults and not block the queue", func() {
			Expect(dPCCNoDiscovery).To(ExecuteSuccessfully())
			Expect(dPCCNoDiscovery.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(dPCCNoDiscovery.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["default"]`))
		})
	})
})
