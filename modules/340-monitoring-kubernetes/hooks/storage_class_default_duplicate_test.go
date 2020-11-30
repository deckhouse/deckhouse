package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: storage_class_default_duplicate ::", func() {
	const (
		singleDefaultStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
  name: main
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: auxiliary
`
		multipleDefaultStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
  name: main
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
  name: auxiliary
`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing single default StorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(singleDefaultStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, StorageClasses must be in cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			scMain := f.KubernetesGlobalResource("StorageClass", "main")
			scAuxiliary := f.KubernetesGlobalResource("StorageClass", "auxiliary")
			Expect(scMain.Exists()).To(BeTrue())
			Expect(scAuxiliary.Exists()).To(BeTrue())
			Expect(scMain.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
			Expect(scAuxiliary.Field("metadata.annotations").Exists()).To(BeFalse())
		})
	})

	Context("Cluster containing multiple default StorageClasses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(multipleDefaultStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, StorageClasses must be in cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			scMain := f.KubernetesGlobalResource("StorageClass", "main")
			scAuxiliary := f.KubernetesGlobalResource("StorageClass", "auxiliary")
			Expect(scMain.Exists()).To(BeTrue())
			Expect(scAuxiliary.Exists()).To(BeTrue())
			Expect(scMain.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
			Expect(scAuxiliary.Field("metadata.annotations").String()).To(MatchYAML(`
storageclass.kubernetes.io/is-default-class: "true"
`))
		})
	})

})
