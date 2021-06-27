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

var _ = Describe("Modules :: prometheus :: hooks :: prometheus_longterm_storage_class_change ::", func() {
	const (
		initValuesString       = `{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }}}`
		initConfigValuesString = `{}`
	)

	const (
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
	f.RegisterCRD("monitoring.coreos.com", "v1", "Prometheus", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
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
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("Cluster with settings up global.storageClass|prometheus.longtermStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.ConfigValuesSet("prometheus.longtermStorageClass", "prometheus-longterm-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-longterm-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("prometheus-longterm-sc"))
		})
	})

	Context("Cluster with PVCs and settings up global.discovery.defaultStorageClass|global.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcLongterm + pvcGrafana))
			f.ConfigValuesSet("global.storageClass", "global-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be pvc-sc-longterm", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("pvc-sc-longterm"))
		})
	})

	Context("Cluster with StatefullSets, PVCs and settings up prometheus.longtermStorageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcLongterm + pvcGrafana + longtermStatefullSet))
			f.ConfigValuesSet("prometheus.longtermStorageClass", "prometheus-longterm-sc")
			f.RunHook()
		})

		It("Must be executed successfully; effectiveStorageClass must be prometheus-longterm-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.effectiveStorageClass").String()).To(Equal("prometheus-longterm-sc"))
		})

		It("PVC must be deleted", func() {
			Expect(f.KubernetesResource("PersistentVolumeClaim", "d8-monitoring", "prometheus-longterm-db-prometheus-longterm-0").Exists()).To(BeFalse())
		})
	})

})
