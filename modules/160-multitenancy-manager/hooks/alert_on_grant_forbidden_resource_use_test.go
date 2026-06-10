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

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: alert_on_grant_forbidden_resource_use ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal: {}
`

	const kubeStateOneViolation = `
apiVersion: v1
kind: Namespace
metadata:
  name: testproj
  labels:
    heritage: multitenancy-manager
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: testreg
spec:
  defaultAvailability: None
  usageReferences:
  - rule:
      apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["configmaps"]
    fieldPath: $.data.scName
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: testgrant
spec:
  projectSelector:
    matchLabels:
      heritage: multitenancy-manager
  resources:
  - resourceName: testreg
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

	// Registration whose excluded is a LIST of filters (the on-disk shape). It must unmarshal and the
	// excluded value must be reported even though the registration baseline is All.
	const kubeStateExcludedList = `
apiVersion: v1
kind: Namespace
metadata:
  name: testproj
  labels:
    heritage: multitenancy-manager
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: excreg
spec:
  defaultAvailability: All
  excluded:
  - names: ["forbidden"]
  usageReferences:
  - rule:
      apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["configmaps"]
    fieldPath: $.data.scName
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: excgrant
spec:
  projectSelector:
    matchLabels:
      heritage: multitenancy-manager
  resources:
  - resourceName: excreg
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: exccm
  namespace: testproj
data:
  scName: forbidden
`

	// Registration baseline is All, but a grant entry carries an allow-list and no explicit
	// availabilityDefault: the allow-list must restrict the resource (baseline None for the rest), so a
	// disallowed value is a violation. This is the common case the alert previously missed.
	const kubeStateAllowListAllBaseline = `
apiVersion: v1
kind: Namespace
metadata:
  name: testproj
  labels:
    heritage: multitenancy-manager
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: allreg
spec:
  defaultAvailability: All
  usageReferences:
  - rule:
      apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["configmaps"]
    fieldPath: $.data.scName
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: allgrant
spec:
  projectSelector:
    matchLabels:
      heritage: multitenancy-manager
  resources:
  - resourceName: allreg
    allowed: ["local"]
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: allcm
  namespace: testproj
data:
  scName: violating
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "ClusterResourceGrantPolicy", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)

	Context("No violations", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook should only expire the shared metric group and publish no violations", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroup,
					Action: operation.ActionExpireMetrics,
				},
			))
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
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroup,
					Action: operation.ActionExpireMetrics,
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroup,
					Labels: map[string]string{
						"grant":                 "testgrant",
						"project":               "testproj",
						"violating_object_name": "testcm",
						"violating_field":       "$.data.scName",
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
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroup,
					Action: operation.ActionExpireMetrics,
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroup,
					Labels: map[string]string{
						"grant":                 "testgrant",
						"project":               "testproj",
						"violating_object_name": "testcm",
						"violating_field":       "$.data.scName",
						"violating_resource":    "configmaps",
					},
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroup,
					Labels: map[string]string{
						"grant":                 "testgrant",
						"project":               "testproj",
						"violating_object_name": "secondcm",
						"violating_field":       "$.data.scName",
						"violating_resource":    "configmaps",
					},
				},
			))
		})
	})

	Context("Excluded as a list", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeStateExcludedList))
			f.RunHook()
		})

		It("Should unmarshal the excluded list and report the excluded value as a violation", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroup,
					Action: operation.ActionExpireMetrics,
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroup,
					Labels: map[string]string{
						"grant":                 "excgrant",
						"project":               "testproj",
						"violating_object_name": "exccm",
						"violating_field":       "$.data.scName",
						"violating_resource":    "configmaps",
					},
				},
			))
		})
	})

	Context("Allow-list with All registration baseline", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeStateAllowListAllBaseline))
			f.RunHook()
		})

		It("Should treat an allow-list as restricting the resource and report the disallowed value", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroup,
					Action: operation.ActionExpireMetrics,
				},
				operation.MetricOperation{
					Action: operation.ActionGaugeSet,
					Name:   grantViolationMetricName,
					Value:  ptr.To(1.0),
					Group:  grantViolationMetricGroup,
					Labels: map[string]string{
						"grant":                 "allgrant",
						"project":               "testproj",
						"violating_object_name": "allcm",
						"violating_field":       "$.data.scName",
						"violating_resource":    "configmaps",
					},
				},
			))
		})
	})
})
