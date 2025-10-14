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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	deprecatedStateLimitNs = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: administrators
spec:
  accessLevel: SuperAdmin
  allowScale: true
  limitNamespaces:
  - dev
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: name
        operator: In
        values:
        - test
        - dev
  portForwarding: true
  subjects:
  - kind: Group
    name: administrators
`
	deprecatedStateLimitSystemNs = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: administrators2
spec:
  accessLevel: SuperAdmin
  allowAccessToSystemNamespaces: false
  allowScale: true
  limitNamespaces:
  - prod
  portForwarding: true
  subjects:
  - kind: Group
    name: administrators
`
	state = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: administrators3
spec:
  accessLevel: SuperAdmin
  allowScale: true
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: name
        operator: In
        values:
        - test
        - dev
  portForwarding: true
  subjects:
  - kind: Group
    name: administrators
`
)

var _ = Describe("User-authz hooks :: alert_deprecated_car_spec ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "ClusterAuthorizationRule", false)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Should have no metrics regarding deprecated clusterAuthorizationRule spec", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("A cluster with a valid CAR", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Should have no metrics regarding deprecated clusterAuthorizationRule spec", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Cluster with a namespace-limited clusterAuthorizationRule", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deprecatedStateLimitNs))
			f.RunHook()
		})

		It("Metrics should be created for all objects with deprecated spec", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_car_spec",
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"kind": "ClusterAuthorizationRule",
					"name": "administrators",
				},
			}))
		})
	})

	Context("Cluster with a system-namespace-limited clusterAuthorizationRule", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deprecatedStateLimitSystemNs))
			f.RunHook()
		})

		It("Metrics should be created for all objects with deprecated spec", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_car_spec",
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"kind": "ClusterAuthorizationRule",
					"name": "administrators2",
				},
			}))
		})
	})

	Context("Cluster with valid and limited clusterAuthorizationRules", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state + deprecatedStateLimitNs + deprecatedStateLimitSystemNs))
			f.RunHook()
		})

		It("Metrics should be created for all objects with deprecated spec", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_car_spec",
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"kind": "ClusterAuthorizationRule",
					"name": "administrators",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_deprecated_car_spec",
				Group:  "d8_deprecated_car_spec",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"kind": "ClusterAuthorizationRule",
					"name": "administrators2",
				},
			}))
		})
	})
})
