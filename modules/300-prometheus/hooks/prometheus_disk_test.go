/*
Copyright 2021 Flant JSC

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

var _ = FDescribe("Modules :: prometheus :: hooks :: prometheus_disk ::", func() {
	const (
		prom = `
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  labels:
    app: prometheus
  name: main
  namespace: d8-monitoring
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  labels:
    app: prometheus
  name: longterm
  namespace: d8-monitoring
`
		pvcMain = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: main
  name: prometheus-main-db-prometheus-main-0
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 15Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 15Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: main
  name: prometheus-main-db-prometheus-main-1
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 45Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 45Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
`
		pvcLt = `
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: longterm
  name: prometheus-longterm-db-prometheus-longterm-0
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 10Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 10Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    app: prometheus
    prometheus: longterm
  name: prometheus-longterm-db-prometheus-longterm-1
  namespace: d8-monitoring
spec:
  resources:
    requests:
      storage: 40Gi
  storageClassName: ceph-ssd
status:
  capacity:
    storage: 40Gi
  conditions:
  - status: "True"
    type: FileSystemResizePending
`
		storageClassExpensionTrue = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
allowVolumeExpansion: true
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: rbd
allowVolumeExpansion: false
`
		storageClassExpensionFalse = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
allowVolumeExpansion: false
`
		pods = `
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-main-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: main
status:
  conditions:
  - status: "True"
    type: PodScheduled
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-main-1
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: main
status:
  conditions:
  - status: "True"
    type: PodScheduled
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-longterm-0
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: longterm
status:
  conditions:
  - status: "True"
    type: PodScheduled
---
apiVersion: v1
kind: Pod
metadata:
  name: prometheus-longterm-1
  namespace: d8-monitoring
  labels:
    app: prometheus
    prometheus: longterm
status:
  conditions:
  - status: "True"
    type: PodScheduled
`
	)

	f := HookExecutionConfigInit(`{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }}}`, `{}`)
	f.RegisterCRD("monitoring.coreos.com", "v1", "Prometheus", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Empty cluster and Schedule", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with storageClassExpensionFalse", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(prom + storageClassExpensionFalse))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.RunHook()
		})

		It("must be executed successfully; main and longterm disk size must be 30, retention must be 27", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("30"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("27"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("30"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("27"))
		})
	})

	Context("Cluster with storageClassExpensionTrue", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(prom + storageClassExpensionTrue))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.RunHook()
		})

		It("must be executed successfully; main and longterm disk size must be 25, retention must be 22", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("25"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("22"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("25"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("22"))
		})
	})

	Context("Cluster with storageClassExpensionTrue and pvc", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(prom + pvcMain + pvcLt + storageClassExpensionTrue))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.ValuesSet("prometheus.internal.prometheusMain.effectiveStorageClass", "ceph-ssd")
			f.ValuesSet("prometheus.internal.prometheusLongterm.effectiveStorageClass", "ceph-ssd")
			f.RunHook()
		})

		It("must be executed successfully; main disk size must be 45, retention must be 36; longterm disk size must be 40, retention must be 32", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("45"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("36"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("40"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("32"))
		})
	})

	Context("Cluster with pvc's in state FileSystemResizePending", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pods + pvcMain + pvcLt + prom))
			f.RunHook()
		})

		It("must be executed successfully; pods must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-main-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-main-1").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-longterm-0").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-monitoring", "prometheus-longterm-1").Exists()).To(BeFalse())
		})
	})

})
