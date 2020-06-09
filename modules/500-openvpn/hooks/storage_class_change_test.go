package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: openvpn :: hooks :: storage_class_change ::", func() {
	const (
		initValuesString       = `{"openvpn": {"internal": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		pvc = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: certs-openvpn-0
  namespace: d8-openvpn
  labels:
    app: openvpn
spec:
  storageClassName: pvc-sc-openvpn
`
		statefulSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: openvpn
  namespace: d8-openvpn
  labels:
    app: openvpn
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
			Expect(f.ValuesGet("openvpn.internal.effectiveStorageClass").String()).To(Equal("false"))
		})
	})

	Context("Cluster with defaultStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultStorageClass))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be default-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.internal.effectiveStorageClass").String()).To(Equal("default-sc"))
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
			Expect(f.ValuesGet("openvpn.internal.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|openvpn.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("openvpn.storageClass", "openvpn-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be openvpn-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.internal.effectiveStorageClass").String()).To(Equal("openvpn-sc"))
		})
	})

	Context("Cluster with PVC and setting up global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be pvc-sc-openvpn", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.internal.effectiveStorageClass").String()).To(Equal("pvc-sc-openvpn"))
		})
	})

	Context("Cluster with PVC, StatefulSet and setting up openvpn.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc + statefulSet))
			f.ConfigValuesSet("openvpn.storageClass", "openvpn-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be openvpn-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("openvpn.internal.effectiveStorageClass").String()).To(Equal("openvpn-sc"))
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-openvpn", "certs-openvpn-0").Exists()).To(BeFalse())
		})
	})

})
