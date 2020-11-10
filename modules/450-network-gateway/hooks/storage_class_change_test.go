package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: network-gateway :: hooks :: storage_class_change ::", func() {
	const (
		initValuesString       = `{"networkGateway": {"internal": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		pvc = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: dhcp-data-dhcp-0
  namespace: d8-network-gateway
  labels:
    app: dhcp
spec:
  storageClassName: sc-from-pvc
`
		pod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: dhcp-0
  namespace: d8-network-gateway
  labels:
    app: dhcp
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
			Expect(f.ValuesGet("networkGateway.internal.effectiveStorageClass").String()).To(Equal("false"))
		})
	})

	Context("Cluster with defaultStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultStorageClass))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be default-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("networkGateway.internal.effectiveStorageClass").String()).To(Equal("default-sc"))
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
			Expect(f.ValuesGet("networkGateway.internal.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|networkGateway.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("networkGateway.storageClass", "sc-from-config")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be sc-from-config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("networkGateway.internal.effectiveStorageClass").String()).To(Equal("sc-from-config"))
		})
	})

	Context("Cluster with PVC and setting up global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be sc-from-pvc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("networkGateway.internal.effectiveStorageClass").String()).To(Equal("sc-from-pvc"))
		})
	})

	Context("Cluster with PVC, Pod and setting up networkGateway.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvc + pod))
			f.ConfigValuesSet("networkGateway.storageClass", "sc-from-config")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be sc-from-config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("networkGateway.internal.effectiveStorageClass").String()).To(Equal("sc-from-config"))
			Expect(f.KubernetesResource("Pod", "d8-network-gateway", "dhcp-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PersistentVolumeClaim", "kube-networkGateway", "networkGateway").Exists()).To(BeFalse())
		})
	})

})
