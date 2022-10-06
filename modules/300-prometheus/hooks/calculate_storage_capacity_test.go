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

var _ = Describe("Modules :: prometheus :: hooks :: calculate_storage_capacity ::", func() {
	const (
		pvcs = `
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
      storage: 70Gi
  storageClassName: test
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
      storage: 55Gi
  storageClassName: test
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
      storage: 40Gi
  storageClassName: test
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
      storage: 50Gi
  storageClassName: test
`

		pvcsLarge = `
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
      storage: 300Gi
  storageClassName: test
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
      storage: 300Gi
  storageClassName: test
`
	)

	f := HookExecutionConfigInit(`{"prometheus": {"internal":{"prometheusMain":{}, "prometheusLongterm":{} }}}`, `{}`)
	f.RegisterCRD("monitoring.coreos.com", "v1", "Prometheus", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("must be executed successfully; main disk size must be 40, retention must be 32; longterm disk size must be 40, retention must be 32", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("40"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("32"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("40"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("32"))
		})
	})

	Context("Cluster with PVCs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcs))
			f.RunHook()
		})

		It("must be executed successfully; main disk size must be 70, retention must be 56; longterm disk size must be 50, retention must be 40", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("70"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("56"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("50"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("40"))
		})
	})

	Context("Cluster with Large PVCs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pvcsLarge))
			f.RunHook()
		})

		It("must be executed successfully; main disk size must be 300, retention must be 250; longterm disk size must be 300, retention must be 250", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.diskSizeGigabytes").String()).To(Equal("300"))
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.retentionGigabytes").String()).To(Equal("250"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.diskSizeGigabytes").String()).To(Equal("300"))
			Expect(f.ValuesGet("prometheus.internal.prometheusLongterm.retentionGigabytes").String()).To(Equal("250"))
		})
	})
})
