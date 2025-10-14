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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
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
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(1))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionExpireMetrics,
			}))
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
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(3))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionExpireMetrics,
			}))

			Expect(ops[1]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_default_duplicate",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "auxiliary",
				},
			}))
			Expect(ops[2]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_default_duplicate",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "main",
				},
			}))

		})
	})

})
