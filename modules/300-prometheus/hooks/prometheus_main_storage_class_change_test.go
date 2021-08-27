/*
Copyright 2021 Flant CJSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: prometheus_main_storage_class_change ::", func() {
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
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("monitoring.coreos.com", "v1", "Prometheus", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("false"))
		})

	})

	Context("Cluster with defaultStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(defaultStorageClass, 1))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be default-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("default-sc"))
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
		})
	})

	Context("Cluster with PVCs and settings up global.discovery.defaultStorageClass|global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(pvcMain+pvcGrafana, 2))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be pvc-sc-main", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("pvc-sc-main"))
		})
	})

	Context("Cluster with StatefullSets, PVCs and settings up prometheus.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcMain + pvcGrafana + mainStatefullSet))
			f.ConfigValuesSet("prometheus.storageClass", "prometheus-main-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-main-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.effectiveStorageClass").String()).To(Equal("prometheus-main-sc"))
		})

		It("PVC must be deleted", func() {
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-main-db-prometheus-main-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-main-db-prometheus-main-1").Exists()).To(BeFalse())
		})
	})

})
