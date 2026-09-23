/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-zvirt :: hooks :: zvirt_cluster_configuration :: discovery data ::", func() {
	const initValues = `
cloudProviderZvirt:
  provider:
    parameters:
      server: https://zvirt.example.com/ovirt-engine/api
      clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
  nodes:
    parameters:
      sshPublicKey: ssh-rsa AAAA
      layout: Standard
  internal: {}
`

	const initValuesWithStorageDomains = `
cloudProviderZvirt:
  provider:
    parameters:
      server: https://zvirt.example.com/ovirt-engine/api
      clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
  nodes:
    parameters:
      sshPublicKey: ssh-rsa AAAA
      layout: Standard
  internal:
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: ZvirtCloudProviderDiscoveryData
      zones: [default]
      storageDomains:
      - name: data
        isEnabled: true
`

	// The new model is in place: an enabled ModuleConfig v2 and the credential Secret.
	const migratedCluster = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  enabled: true
  version: 2
  settings:
    provider:
      parameters:
        server: https://zvirt.example.com/ovirt-engine/api
        clusterID: b46372e7-0d52-40c7-9bbf-fda31e187088
    nodes:
      parameters:
        sshPublicKey: ssh-rsa AAAA
        layout: Standard
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-zvirt
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: dXNlclBhc3N3b3Jk
  identity: YWRtaW5AaW50ZXJuYWw=
  secret: cGFzc3dvcmQ=
`

	secretWithDiscoveryData := func(name, namespace, discoveryData string) string {
		return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
data:
  cloud-provider-discovery-data.json: %s
`, name, namespace, base64.StdEncoding.EncodeToString([]byte(discoveryData)))
	}

	candiSecret := func(discoveryData string) string {
		return secretWithDiscoveryData("d8-candi-cloud-provider-discovery-data", "d8-cloud-provider-zvirt", discoveryData)
	}

	newHook := func(values string) *HookExecutionConfig {
		f := HookExecutionConfigInit(values, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)
		f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
		f.RegisterCRD("deckhouse.io", "v1", "ZvirtInstanceClass", false)
		return f
	}

	discoveryDataValue := func(f *HookExecutionConfig) string {
		return f.ValuesGet("cloudProviderZvirt.internal.providerDiscoveryData").String()
	}

	Context("A cluster bootstrapped with the ModuleConfig: dhctl recorded the discovery data in the candi Secret", func() {
		f := newHook(initValues)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedCluster + candiSecret(
				`{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["zone-a"],"storageDomains":[]}`,
			)))
			f.RunHook()
		})

		It("publishes the candi discovery data", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(discoveryDataValue(f)).To(MatchJSON(`
{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["zone-a"]}`))
		})
	})

	Context("A hybrid cluster: neither the candi Secret nor the PCC Secret exists", func() {
		f := newHook(initValues)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedCluster))
			f.RunHook()
		})

		// templates/registration.yaml requires the discovery data. Without the defaults the release
		// would not render, and cloud-data-discoverer — the only other source — would never start.
		It("publishes the defaults, so the module renders", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(discoveryDataValue(f)).To(MatchJSON(`
{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["default"]}`))
		})
	})

	Context("The candi Secret exists but dhctl has not filled it in yet", func() {
		f := newHook(initValues)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedCluster + `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-candi-cloud-provider-discovery-data
  namespace: d8-cloud-provider-zvirt
data: {}
`))
			f.RunHook()
		})

		It("falls back to the defaults", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(discoveryDataValue(f)).To(MatchJSON(`
{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["default"]}`))
		})
	})

	Context("Both the candi Secret and a legacy PCC Secret carry discovery data", func() {
		f := newHook(initValues)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedCluster +
				candiSecret(`{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["from-candi"]}`) +
				secretWithDiscoveryData("d8-provider-cluster-configuration", "kube-system",
					`{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["from-pcc"]}`),
			))
			f.RunHook()
		})

		It("prefers the candi Secret, as dhctl writes it on every run of a ModuleConfig cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(discoveryDataValue(f)).To(MatchJSON(`
{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["from-candi"]}`))
		})
	})

	Context("Only the legacy PCC Secret carries discovery data", func() {
		f := newHook(initValues)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedCluster +
				secretWithDiscoveryData("d8-provider-cluster-configuration", "kube-system",
					`{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["from-pcc"]}`),
			))
			f.RunHook()
		})

		It("falls back to the PCC discovery data", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(discoveryDataValue(f)).To(MatchJSON(`
{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["from-pcc"]}`))
		})
	})

	Context("cloud-data-discoverer has already published the storage domains", func() {
		f := newHook(initValuesWithStorageDomains)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migratedCluster + candiSecret(
				`{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["default"],"storageDomains":[]}`,
			)))
			f.RunHook()
		})

		// Neither dhctl nor the PCC carries storage domains; overwriting them with the empty list
		// would drop every StorageClass until the discoverer's next run.
		It("keeps them", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(discoveryDataValue(f)).To(MatchJSON(`
{"apiVersion":"deckhouse.io/v1","kind":"ZvirtCloudProviderDiscoveryData","zones":["default"],
 "storageDomains":[{"name":"data","isEnabled":true}]}`))
		})
	})
})
