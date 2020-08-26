package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: upmeter :: hooks :: storage_class_change ::", func() {
	const (
		initValuesString       = `{"upmeter": {"internal": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		pvc = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-upmeter-0
  namespace: d8-upmeter
  labels:
    app: upmeter
spec:
  storageClassName: pvc-sc-upmeter
`
		statefulSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: upmeter
  namespace: d8-upmeter
  labels:
    app: upmeter
`
		defaultStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("upmeter.internal.effectiveStorageClass").String()).To(Equal("false"))
		})
	})

	Context("Cluster with defaultStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultStorageClass))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be default-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("upmeter.internal.effectiveStorageClass").String()).To(Equal("default-sc"))
		})
	})

	Context("Cluster with setting up global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be global-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("upmeter.internal.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|upmeter.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("upmeter.storageClass", "upmeter-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be upmeter-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("upmeter.internal.effectiveStorageClass").String()).To(Equal("upmeter-sc"))
		})
	})

	Context("Cluster with PVC and setting up global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be pvc-sc-upmeter", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("upmeter.internal.effectiveStorageClass").String()).To(Equal("pvc-sc-upmeter"))
		})
	})

	Context("Cluster with PVC, StatefulSet and setting up upmeter.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc + statefulSet))
			f.ConfigValuesSet("upmeter.storageClass", "upmeter-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be upmeter-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("upmeter.internal.effectiveStorageClass").String()).To(Equal("upmeter-sc"))
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-upmeter", "data-upmeter-0").Exists()).To(BeFalse())
		})
	})

})
