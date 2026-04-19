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

var _ = Describe("Modules :: common :: hooks :: storage_class_change ::", func() {

	f := HookExecutionConfigInit(`{"common": {"internal":{"testSubPath":{}}}}`, `{}`)

	Context("empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("effectiveStorageClass must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("false"))
		})

	})

	Context("cluster with default storage class", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
`))
			f.RunHook()
		})

		It("effectiveStorageClass must be default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("default"))
		})
	})

	Context("global.modules.storageClass is defined", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("global.modules.storageClass", "global-sc")
			f.RunHook()
		})

		It("effectiveStorageClass must be global-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("global-sc"))
		})
	})

	Context("common.storageClass is defined", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ConfigValuesSet("common.testStorageClass", "common-sc")
			f.RunHook()
		})

		It("effectiveStorageClass must be common-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("common-sc"))
		})
	})

	Context("cluster with PersistentVolumeClaim", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-0
  namespace: d8-module-name
  labels:
    app: test
    test: test
spec:
  storageClassName: pvc-sc
`))
			f.RunHook()
		})

		It("effectiveStorageClass must be pvc-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("pvc-sc"))
		})
	})

	Context("cluster with PersistentVolumeClaim and default storage class", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-0
  namespace: d8-module-name
  labels:
    app: test
    test: test
spec:
  storageClassName: pvc-sc
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
`))
			f.RunHook()
		})

		It("effectiveStorageClass must be pvc-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("pvc-sc"))
		})
	})

	Context("cluster with PersistentVolumeClaim and new value defined in global.modules.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-0
  namespace: d8-module-name
  labels:
    app: test
    test: test
spec:
  storageClassName: pvc-sc
`))
			f.ConfigValuesSet("global.modules.storageClass", "global-sc")
			f.RunHook()
		})

		It("effectiveStorageClass must be pvc-sc", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("pvc-sc"))
		})
	})

	Context("cluster with PersistentVolumeClaim and new value defined in common.storageClass", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-0
  namespace: d8-module-name
  labels:
    app: test
    test: test
spec:
  storageClassName: pvc-sc
`))
			f.ConfigValuesSet("common.testStorageClass", "false")
			f.RunHook()
		})

		It("effectiveStorageClass must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.testSubPath.effectiveStorageClass").String()).To(Equal("false"))
		})

		It("Should set d8_emptydir_usage metric to 1 (because AllowEmptyDir is not set)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1))

			metric := f.MetricsCollector.CollectedMetrics()[0]
			Expect(metric.Name).To(Equal("d8_emptydir_usage"))
			Expect(*metric.Value).To(Equal(float64(1)))
			Expect(metric.Labels["namespace"]).To(Equal("d8-module-name"))
			Expect(metric.Labels["module_name"]).To(Equal("common"))
		})
	})

})
