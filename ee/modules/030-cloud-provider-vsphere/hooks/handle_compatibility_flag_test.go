/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: handle_compatibility_flag ::", func() {
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

		It("Hook must not fail, storage class should be present", func() {
			Expect(f).To(ExecuteSuccessfully())

		})
	})

	Context("Cluster with StorageClasses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(manualDefaultStorageClass + moduleStorageClass))

			f.RunHook()
		})

		It("Hook must not fail, storage class should be present, but should not be default", func() {
			Expect(f).To(ExecuteSuccessfully())

			scManual := f.KubernetesGlobalResource("StorageClass", "vsphere-main")
			Expect(scManual.Exists()).To(BeTrue())

			scLeegacyModule := f.KubernetesGlobalResource("StorageClass", "test-lun001-02baf966-legacy")
			Expect(scLeegacyModule.Exists()).To(BeFalse())

			scModule := f.KubernetesGlobalResource("StorageClass", "test-lun001-02baf966")
			Expect(scModule.Exists()).To(BeTrue())
		})
	})
})
