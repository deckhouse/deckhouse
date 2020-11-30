package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: migrate_default_storage_class ::", func() {
	const (
		initValues = `
global:
  discovery: {}
cloudProviderVsphere:
  internal: {}
`
		initConfigValues = `
cloudProviderVsphere: {}
`
		manualDefaultStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
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
  name: test-lun001-02baf966
parameters:
  parent_name: /DCTEST/datastore/test_lun_001
  parent_type: Datastore
provisioner: vsphere.csi.vmware.com
`
	)

	f := HookExecutionConfigInit(initValues, initConfigValues)

	Context("Fresh cluster without StorageClasses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with manual default StorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(manualDefaultStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, storage class should be present as a default", func() {
			Expect(f).To(ExecuteSuccessfully())
			scManual := f.KubernetesGlobalResource("StorageClass", "vsphere-main")
			Expect(scManual.Exists()).To(BeTrue())
			Expect(scManual.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

	Context("Cluster with module StorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(moduleStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, storage class should be present, but should not be default", func() {
			Expect(f).To(ExecuteSuccessfully())
			scModule := f.KubernetesGlobalResource("StorageClass", "test-lun001-02baf966")
			Expect(scModule.Exists()).To(BeTrue())
			Expect(scModule.Field("metadata.annotations").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with StorageClasses suitable for migration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(manualDefaultStorageClass + moduleStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, manual StorageClass should be deleted and module StorageClass should be set as default, in config as well", func() {
			Expect(f).To(ExecuteSuccessfully())
			scManualDefault := f.KubernetesGlobalResource("StorageClass", "vsphere-main")
			scManual := f.KubernetesGlobalResource("StorageClass", "test-lun001-02baf966")

			Expect(scManualDefault.Exists()).To(BeFalse())
			Expect(scManual.Exists()).To(BeTrue())

			Expect(f.ConfigValuesGet("cloudProviderVsphere.storageClass.default").String()).To(Equal(`test-lun001-02baf966`))
		})
	})
})
