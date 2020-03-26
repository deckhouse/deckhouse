package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: prometheus_storage_class_change ::", func() {
	const (
		initValuesString       = `{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }}}`
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
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-main-db-prometheus-main-1
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
		mainStatefullSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: prometheus-main
  namespace: d8-monitoring
`
		longtermStatefullSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: prometheus-longterm
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
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("false"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("false"))
		})

	})

	Context("Cluster with defaultStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultStorageClass))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be default-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("default-sc"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("default-sc"))
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
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("global-sc"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|prometheus.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("prometheus.storageClass", "prometheus-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("prometheus-sc"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("prometheus-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|prometheus.storageClass|prometheus.longtermStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("prometheus.storageClass", "prometheus-main-sc")
			f.ConfigValuesSet("prometheus.longtermStorageClass", "prometheus-longterm-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-main-sc and prometheus-longterm-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("prometheus-main-sc"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("prometheus-longterm-sc"))
		})
	})

	Context("Cluster with PVCs and settings up global.discovery.defaultStorageClass|global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcLongterm + pvcGrafana))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be pvc-sc-main and pvc-sc-longterm", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("pvc-sc-main"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("pvc-sc-longterm"))
		})
	})

	Context("Cluster with StatefullSets, PVCs and settings up prometheus.storageClass and prometheus.longtermStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcLongterm + pvcGrafana + mainStatefullSet + longtermStatefullSet))
			f.ConfigValuesSet("prometheus.storageClass", "prometheus-main-sc")
			f.ConfigValuesSet("prometheus.longtermStorageClass", "prometheus-longterm-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-main-sc and prometheus-longterm-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("prometheus-main-sc"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("prometheus-longterm-sc"))
		})

		It("StatefullSets prometheus-main and prometheus-longterm and their pvc must be deleted", func() {
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-main-db-prometheus-main-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-main-db-prometheus-main-1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-longterm-db-prometheus-longterm-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("StatefulSet", "d8-monitoring", "prometheus-main").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("StatefulSet", "d8-monitoring", "prometheus-longterm").Exists()).To(BeFalse())
		})
	})

})
