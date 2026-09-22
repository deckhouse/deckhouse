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

// The periodic re-scan is the hook that actually publishes the violation series between policy
// events, and until the two registrations were split into separate files it was the only one of
// the pair that kept its bindings, while this suite's sibling covered the other handler. Cover it
// on its own schedule binding so neither half can go dark unnoticed again.
var _ = Describe("Modules :: multitenancy-manager :: hooks :: scan_grant_forbidden_resource_use ::", func() {
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
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: testref
spec:
  grantableClusterResourceName: testreg
  rule:
    apiGroups: [""]
    apiVersions: ["v1"]
    resources: ["configmaps"]
  fieldPaths:
  - path: $.data.scName
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

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "ClusterResourceGrantPolicy", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceReference", false)

	Context("Empty cluster on schedule", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/2 * * * *"))
			f.RunGoHook()
		})

		It("Expires the shared metric group and publishes no violations", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(
				operation.MetricOperation{
					Group:  grantViolationMetricGroup,
					Action: operation.ActionExpireMetrics,
				},
			))
		})
	})

	Context("One violation on schedule", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(kubeStateOneViolation))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/2 * * * *"))
			f.RunGoHook()
		})

		// The handler lists the policies itself instead of reading a snapshot, so the schedule
		// binding alone has to be enough to produce the series.
		It("Reports the violation without any Kubernetes binding of its own", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(
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
})
