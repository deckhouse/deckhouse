/*
Copyright 2026 Flant JSC

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

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: alert_on_grant_forbidden_resource_use ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal: {}
`

	const kubeStateOneViolation = `
apiVersion: projects.deckhouse.io/v1alpha1
kind: ClusterObjectGrantPolicy
metadata:
  name: testpolicy
spec:
  grantedResource:
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
  usageReferences:
  - apiVersion: "v1"
    fieldPath: $.data.scName
    resource: configmaps
---
apiVersion: projects.deckhouse.io/v1alpha1
kind: ClusterObjectsGrant
metadata:
  name: testproj
spec:
  clusterObjectGrantPolicies:
  - name: testpolicy
    allowed: ["local", "abcd"]
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: testcm
  namespace: testproj
data:
  scName: violating
`

	const kubeStateManyViolations = kubeStateOneViolation + `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: secondcm
  namespace: testproj
data:
  scName: other-violating
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("projects.deckhouse.io", "v1alpha1", "ClusterObjectsGrant", false)
	f.RegisterCRD("projects.deckhouse.io", "v1alpha1", "ClusterObjectGrantPolicy", false)

	Context("No violations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should not publish any violations in metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(0))
		})
	})

	Context("One violation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeStateOneViolation))
			f.RunHook()
		})

		It("Should detect and report one violation in metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(2))
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroupPrefix + "testproj",
					Action: operation.ActionExpireMetrics,
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroupPrefix + "testproj",
					Labels: map[string]string{
						"project":               "testproj",
						"violating_object_name": "testcm",
						"violating_resource":    "configmaps",
					},
				}))
		})
	})

	Context("Multiple violations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeStateManyViolations))
			f.RunHook()
		})

		It("Should detect and report 2 violations in metrics", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(3))
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroupPrefix + "testproj",
					Action: operation.ActionExpireMetrics,
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroupPrefix + "testproj",
					Labels: map[string]string{
						"project":               "testproj",
						"violating_object_name": "testcm",
						"violating_resource":    "configmaps",
					},
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroupPrefix + "testproj",
					Labels: map[string]string{
						"project":               "testproj",
						"violating_object_name": "secondcm",
						"violating_resource":    "configmaps",
					},
				},
			))
		})
	})
})
