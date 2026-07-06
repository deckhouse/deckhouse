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

const (
	// CRB to a deprecated manage role — must be flagged.
	crbDeprecatedManage = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: legacy-observability
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:observability:manager
subjects:
- kind: Group
  name: ops
  apiGroup: rbac.authorization.k8s.io
`
	// RoleBinding to a deprecated use role — must be flagged (namespace label present).
	rbDeprecatedUse = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: legacy-viewer
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:role:viewer
subjects:
- kind: User
  name: alice@example.com
  apiGroup: rbac.authorization.k8s.io
`
	// CRB to a NEW-model role — must NOT be flagged.
	crbNewModel = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: modern-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:system:viewer
subjects:
- kind: Group
  name: ops
  apiGroup: rbac.authorization.k8s.io
`
	// RoleBinding to an ordinary (non-d8) ClusterRole — must NOT be flagged.
	rbOrdinary = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-view
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
subjects:
- kind: User
  name: bob@example.com
  apiGroup: rbac.authorization.k8s.io
`
)

var _ = Describe("User-authz hooks :: alert_deprecated_rbacv2_bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Emits only the expire operation", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Only new-model and ordinary bindings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbNewModel + rbOrdinary))
			f.RunHook()
		})

		It("Flags nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).To(BeEquivalentTo(operation.ActionExpireMetrics))
		})
	})

	Context("A ClusterRoleBinding to a deprecated manage role", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbDeprecatedManage + crbNewModel + rbOrdinary))
			f.RunHook()
		})

		It("Flags exactly the deprecated CRB", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).To(BeEquivalentTo(operation.ActionExpireMetrics))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   deprecatedRBACv2Metric,
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"binding_kind": "ClusterRoleBinding",
					"binding_name": "legacy-observability",
					"namespace":    "",
					"role_name":    "d8:manage:observability:manager",
				},
			}))
		})
	})

	Context("A RoleBinding to a deprecated use role", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rbDeprecatedUse))
			f.RunHook()
		})

		It("Flags the deprecated RB with its namespace", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   deprecatedRBACv2Metric,
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"binding_kind": "RoleBinding",
					"binding_name": "legacy-viewer",
					"namespace":    "team-a",
					"role_name":    "d8:use:role:viewer",
				},
			}))
		})
	})
})
