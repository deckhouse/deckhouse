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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const stateLegacyCustomCapabilities = `
---
# counted: the capability of the FAQ example extends d8:use:role:user through the old labels
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:use:capability:mycustom:superresource:view
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
rules:
  - apiGroups: ["deckhouse.io"]
    resources: ["mysuperresources"]
    verbs: ["get", "list", "watch"]
---
# counted: a frozen role with the same labels extends the namespace role the same way
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:use:frozen:manager
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
rules: []
---
# NOT counted: relabeled, it extends the namespace roles of the new scheme too
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:use:capability:relabeled:view
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
    rbac.deckhouse.io/aggregate-to-namespace-as: user
rules: []
---
# NOT counted: a role that aggregates, counted by the legacy custom role alert instead
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:use:role:aggregating
  labels:
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: use
rules: []
---
# NOT counted: a built-in capability
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:use:capability:module:foo:view
  labels:
    heritage: deckhouse
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-kubernetes-as: user
rules: []
---
# NOT counted: a manage capability, which the subsystem roles of the new scheme still select
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:permission:mycustom:superresource:view
  labels:
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-deckhouse-as: viewer
rules: []
`

var _ = Describe("User Authz hooks :: discovery legacy custom capabilities ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("emits only the group expiration", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_custom_capability",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Cluster with legacy, relabeled, aggregating and built-in objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLegacyCustomCapabilities))
			f.RunHook()
		})

		It("emits one alert metric per capability that extends a namespace role through the old labels alone", func() {
			Expect(f).To(ExecuteSuccessfully())
			set := func(name, level string) operation.MetricOperation {
				return operation.MetricOperation{
					Name:   "d8_rbacv2_legacy_custom_capability",
					Group:  "d8_rbacv2_legacy_custom_capability",
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{"name": name, "level": level},
				}
			}
			Expect(f.MetricsCollector.CollectedMetrics()).To(ConsistOf(
				operation.MetricOperation{Group: "d8_rbacv2_legacy_custom_capability", Action: operation.ActionExpireMetrics},
				set("custom:use:capability:mycustom:superresource:view", "user"),
				set("custom:use:frozen:manager", "manager"),
			))
		})
	})
})
