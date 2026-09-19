/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-zvirt :: hooks :: storage_classes ::", func() {
	initValues := `
cloudProviderZvirt:
  internal: {}
`

	// Two classes created by this module, one created by another, and one created by hand: only
	// the module's own are labelled, and only those the hook may touch.
	storageClasses := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-zvirt
provisioner: csi.zvirt.deckhouse.io
parameters:
  storageDomainName: "SAS"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: localpath-system
  labels:
    heritage: deckhouse
    module: local-path-provisioner
provisioner: deckhouse.io/localpath-system
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: hdd
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-zvirt
provisioner: csi.zvirt.deckhouse.io
parameters:
  storageDomainName: "HDD"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	manualStorageClasses := `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-default
provisioner: csi.zvirt.deckhouse.io
parameters:
  storageDomainName: "MANUAL-DEFAULT"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	discoveryDataValues := `
cloudProviderZvirt:
  internal:
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: ZvirtCloudProviderDiscoveryData
      storageDomains:
      - name: D1
        isEnabled: true
      - name: D2
        isEnabled: false
      - name: D3
        isEnabled: true
`

	a := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has nothing", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		// With no discovery data and no classes of its own, the module knows nothing. Publishing
		// an empty list here would delete every StorageClass it had created.
		It("Should leave the values alone", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.ValuesGet("cloudProviderZvirt.internal.storageClasses").Exists()).To(BeFalse())
		})
	})

	b := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has storage classes but no discovery data", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(storageClasses + manualStorageClasses))
			b.RunHook()
		})

		// Values patches do not survive a Deckhouse restart, so the module's own classes are the
		// only source of truth left. Classes of other modules and hand-made ones are not ours.
		It("Should rebuild the classes from the ones it created", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderZvirt.internal.storageClasses").String()).To(MatchJSON(`
[
  {"name": "default", "storageDomain": "SAS", "allowVolumeExpansion": false},
  {"name": "hdd", "storageDomain": "HDD", "allowVolumeExpansion": false}
]
`))
		})
	})

	c := HookExecutionConfigInit(discoveryDataValues, `{}`)
	Context("Discovery data is in values", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(``))
			c.RunHook()
		})

		// Disabled storage domains are not offered, and the names are normalized to RFC 1123.
		It("Should create a class per enabled storage domain", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("cloudProviderZvirt.internal.storageClasses").String()).To(MatchJSON(`
[
  {"name": "d1", "storageDomain": "D1", "allowVolumeExpansion": true},
  {"name": "d3", "storageDomain": "D3", "allowVolumeExpansion": true}
]
`))
		})
	})

	d := HookExecutionConfigInit(discoveryDataValues+`
  storage:
    parameters:
      excludedStorageClasses:
      - d3
      - bar
`, `{}`)
	Context("Some storage classes are excluded", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(``))
			d.RunHook()
		})

		It("Should drop the excluded ones", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("cloudProviderZvirt.internal.storageClasses").String()).To(MatchJSON(`
[
  {"name": "d1", "storageDomain": "D1", "allowVolumeExpansion": true}
]
`))
		})
	})

	e := HookExecutionConfigInit(discoveryDataValues, `{}`)
	Context("A discovered class already exists in the cluster", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(`
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: d1
  labels:
    heritage: deckhouse
    module: cloud-provider-zvirt
provisioner: csi.zvirt.deckhouse.io
parameters:
  storageDomainName: "D1"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`))
			e.RunHook()
		})

		// allowVolumeExpansion is not part of the discovery data, so an existing class keeps the
		// value it was created with instead of being recreated with the default.
		It("Should keep its allowVolumeExpansion", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderZvirt.internal.storageClasses").String()).To(MatchJSON(`
[
  {"name": "d1", "storageDomain": "D1", "allowVolumeExpansion": false},
  {"name": "d3", "storageDomain": "D3", "allowVolumeExpansion": true}
]
`))
		})
	})
})
