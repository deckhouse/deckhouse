package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: grafana_storage_class_change ::", func() {
	const (
		initValuesString       = `{"prometheus": {"internal": {"grafana": {}}}}`
		initConfigValuesString = `{}`
	)

	const (
		pvcMain = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-main-db-prometheus-main-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: main
spec:
  storageClassName: pvc-sc-main
`
		pvcLongterm = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-longterm-db-prometheus-longterm-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: longterm
spec:
  storageClassName: pvc-sc-longterm
`
		pvcGrafana = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: grafana-storage-grafana-0
  namespace: d8-monitoring
  labels:
    app: grafana
spec:
  storageClassName: pvc-sc-grafana
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
		grafanaStatefulSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: grafana
  namespace: d8-monitoring
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
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("false"))
		})
	})

	Context("Cluster with defaultStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultStorageClass))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be default-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("default-sc"))
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
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|grafana.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("prometheus.grafana.storageClass", "grafana-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be grafana-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("grafana-sc"))
		})
	})

	Context("Cluster with prometheus PVCs and settings up global.discovery.defaultStorageClass|global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcLongterm))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("prometheus.storageClass", "prometheus-main-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-main-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("prometheus-main-sc"))
		})
	})

	Context("Cluster with PVCs and settings up global.discovery.defaultStorageClass|global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcLongterm + pvcGrafana))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be pvc-sc-grafana", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("pvc-sc-grafana"))
		})
	})

	Context("Cluster with PVCs and setting up prometheus.grafana.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcLongterm + pvcGrafana + grafanaStatefulSet))
			f.ConfigValuesSet("prometheus.grafana.storageClass", "grafana-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be grafana-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.effectiveStorageClass").String()).To(Equal("grafana-sc"))
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "grafana-storage-grafana-0").Exists()).To(BeFalse())
		})
	})

})
