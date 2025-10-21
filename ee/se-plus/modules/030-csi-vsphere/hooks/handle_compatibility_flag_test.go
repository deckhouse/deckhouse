/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: csi-vsphere :: hooks :: handle_compatibility_flag ::", func() {
	const (
		initValues = `
global:
  discovery: {}
csiVsphere:
  internal: {}
`
		initConfigValues = `
csiVsphere: {}
`
		manualStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vsphere-main
parameters:
  parent_name: test_lun_001
  parent_type: Datastore
provisioner: vsphere.csi.vmware.com
`
		moduleStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: test-lun001-02baf966-legacy
parameters:
  parent_name: /DCTEST/datastore/test_lun_001
  parent_type: Datastore
provisioner: vsphere.csi.vmware.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: test-lun001-02baf966
parameters:
  foo: bar
provisioner: csi.vsphere.vmware.com
`
	)

	f := HookExecutionConfigInit(initValues, initConfigValues)

	Context("Fresh cluster without StorageClasses", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("csiVsphere.internal.compatibilityFlag").String()).To(Equal("none"))
		})
	})

	Context("Cluster with manual default StorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(manualStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, manual storage class should be present", func() {
			Expect(f).To(ExecuteSuccessfully())

			scManual := f.KubernetesGlobalResource("StorageClass", "vsphere-main")
			Expect(scManual.Exists()).To(BeTrue())
		})
	})

	Context("Cluster with StorageClasses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(manualStorageClass + moduleStorageClass))

			f.RunHook()
		})

		It("Hook must not fail, manual and module storage class should be present, but legacy one should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			scManual := f.KubernetesGlobalResource("StorageClass", "vsphere-main")
			Expect(scManual.Exists()).To(BeTrue())

			scLegacyModule := f.KubernetesGlobalResource("StorageClass", "test-lun001-02baf966-legacy")
			Expect(scLegacyModule.Exists()).To(BeFalse())

			scModule := f.KubernetesGlobalResource("StorageClass", "test-lun001-02baf966")
			Expect(scModule.Exists()).To(BeTrue())
		})
	})
})
