/*
Copyright 2023 Flant JSC

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
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	state = `
---
apiVersion: v1
kind: Node
metadata:
  name: test
  annotations:
    extended-monitoring.flant.com/enabled: ""
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
  namespace: default
  annotations:
    extended-monitoring.flant.com/enabled: ""
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test
  namespace: default
  annotations:
    extended-monitoring.flant.com/enabled: ""
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: test
  namespace: default
  annotations:
    extended-monitoring.flant.com/enabled: ""
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: test
  namespace: default
  annotations:
    extended-monitoring.flant.com/enabled: ""
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test
  namespace: default
  annotations:
    extended-monitoring.flant.com/enabled: ""
`
)

var _ = Describe("Extended Monitoring hooks :: alert_old_annotation ::", func() {
	f := HookExecutionConfigInit(``, ``)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should have no metrics regarding deprecated annotation", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_legacy_annotation",
				Action: "expire",
			}))
		})
	})

	Context("Cluster with old annotated Node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Metrics should be created for all deprecated objects", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(7))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_legacy_annotation",
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_legacy_annotation",
				Group:  "d8_deprecated_legacy_annotation",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"kind":      "Node",
					"namespace": "",
					"name":      "test",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_legacy_annotation",
				Group:  "d8_deprecated_legacy_annotation",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"kind":      "Deployment",
					"namespace": "default",
					"name":      "test",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_legacy_annotation",
				Group:  "d8_deprecated_legacy_annotation",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"kind":      "StatefulSet",
					"namespace": "default",
					"name":      "test",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_legacy_annotation",
				Group:  "d8_deprecated_legacy_annotation",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"kind":      "DaemonSet",
					"namespace": "default",
					"name":      "test",
				},
			}))
			Expect(m[5]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_legacy_annotation",
				Group:  "d8_deprecated_legacy_annotation",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"kind":      "CronJob",
					"namespace": "default",
					"name":      "test",
				},
			}))
			Expect(m[6]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_legacy_annotation",
				Group:  "d8_deprecated_legacy_annotation",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"kind":      "Ingress",
					"namespace": "default",
					"name":      "test",
				},
			}))
		})
	})
})
