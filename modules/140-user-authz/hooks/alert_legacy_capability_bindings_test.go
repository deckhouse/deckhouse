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

const stateLegacyCapabilityBindings = `
---
# counted: a system capability of a module
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cert-manager-operators
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:module:cert-manager:edit
subjects:
  - kind: Group
    name: cert-operators
    apiGroup: rbac.authorization.k8s.io
---
# counted: a namespace capability of a module, in a namespace
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: deckhouse-viewers
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:capability:module:deckhouse:view
subjects:
  - kind: User
    name: jane@example.com
    apiGroup: rbac.authorization.k8s.io
---
# counted: a namespace capability of user-authz
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: log-readers
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:capability:kubernetes:view_logs
subjects:
  - kind: Group
    name: developers
    apiGroup: rbac.authorization.k8s.io
---
# counted: a subsystem capability of user-authz, whose subsystem is merged into cluster
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: common-resources-managers
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:subsystem:kubernetes:manage_resources
subjects:
  - kind: Group
    name: platform
    apiGroup: rbac.authorization.k8s.io
---
# counted: a subsystem capability of user-authz, whose subsystem is renamed
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-viewers
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:subsystem:networking:view_resources
subjects:
  - kind: Group
    name: network
    apiGroup: rbac.authorization.k8s.io
---
# NOT counted: Deckhouse created the binding
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:built-in-capability-binding
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:module:user-authz:view
subjects:
  - kind: Group
    name: auditors
    apiGroup: rbac.authorization.k8s.io
---
# NOT counted: a capability of a module installed from a module source, renamed by its own release
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: trivy-viewers
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:module:operator-trivy:view
subjects:
  - kind: Group
    name: security
    apiGroup: rbac.authorization.k8s.io
---
# NOT counted: multitenancy-manager has no such namespace capability
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: projects-editors
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:capability:module:multitenancy-manager:edit
subjects:
  - kind: Group
    name: developers
    apiGroup: rbac.authorization.k8s.io
---
# NOT counted: a namespaced Role with the name of a capability
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: local-role
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: d8:use:capability:kubernetes:view_logs
subjects:
  - kind: Group
    name: developers
    apiGroup: rbac.authorization.k8s.io
---
# NOT counted: a role, which the new role model keeps as an alias
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: all-viewers
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:all:viewer
subjects:
  - kind: Group
    name: viewers
    apiGroup: rbac.authorization.k8s.io
---
# NOT counted: already the new name
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: new-name
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:namespace-capability:kubernetes:view_logs
subjects:
  - kind: Group
    name: developers
    apiGroup: rbac.authorization.k8s.io
`

var _ = Describe("User Authz hooks :: alert legacy capability bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	It("renames every capability of the modules shipped with Deckhouse and of user-authz", func() {
		Expect(legacyCapabilityRenames).To(HaveLen(116))
		Expect(legacyCapabilityRenames).To(HaveKeyWithValue(
			"d8:manage:permission:module:vertical-pod-autoscaler:edit", "d8:system-capability:vertical-pod-autoscaler:edit"))
		Expect(legacyCapabilityRenames).To(HaveKeyWithValue(
			"d8:use:capability:module:multitenancy-manager:view", "d8:namespace-capability:multitenancy-manager:view"))
		Expect(legacyCapabilityRenames).To(HaveKeyWithValue(
			"d8:manage:permission:subsystem:security:manage_resources", "d8:subsystem-capability:security:manage_resources"))
		Expect(legacyCapabilityRenames).To(HaveKeyWithValue(
			"d8:manage:permission:subsystem:storage:view_resources", "d8:subsystem-capability:storage:view_resources"))
		Expect(legacyCapabilityRenames).To(HaveKeyWithValue(
			"d8:use:capability:kubernetes:access_terminal", "d8:namespace-capability:kubernetes:access_terminal"))
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("emits no alert metrics, only the group expiration", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_capability_binding",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Cluster with bindings to renamed capabilities, other roles and the new names", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLegacyCapabilityBindings))
			f.RunHook()
		})

		It("emits one alert metric per binding to a renamed capability, with its new name", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(6))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_capability_binding",
				Action: operation.ActionExpireMetrics,
			}))
			gauge := func(kind, name, namespace, role, replacedBy string) operation.MetricOperation {
				return operation.MetricOperation{
					Name:   "d8_rbacv2_legacy_capability_binding",
					Group:  "d8_rbacv2_legacy_capability_binding",
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{
						"binding_kind": kind,
						"binding_name": name,
						"namespace":    namespace,
						"role_name":    role,
						"replaced_by":  replacedBy,
					},
				}
			}
			Expect(m[1:]).To(ConsistOf(
				gauge("ClusterRoleBinding", "cert-manager-operators", "",
					"d8:manage:permission:module:cert-manager:edit", "d8:system-capability:cert-manager:edit"),
				gauge("ClusterRoleBinding", "common-resources-managers", "",
					"d8:manage:permission:subsystem:kubernetes:manage_resources", "d8:subsystem-capability:cluster:manage_resources"),
				gauge("ClusterRoleBinding", "network-viewers", "",
					"d8:manage:permission:subsystem:networking:view_resources", "d8:subsystem-capability:network:view_resources"),
				gauge("RoleBinding", "deckhouse-viewers", "team-a",
					"d8:use:capability:module:deckhouse:view", "d8:namespace-capability:deckhouse:view"),
				gauge("RoleBinding", "log-readers", "team-a",
					"d8:use:capability:kubernetes:view_logs", "d8:namespace-capability:kubernetes:view_logs"),
			))
		})
	})
})
