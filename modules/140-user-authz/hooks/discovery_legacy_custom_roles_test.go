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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const stateLegacyCustomRoles = `
---
# built-in: the cluster subsystem collects its lineage
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:subsystem:cluster:manager
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/kind: role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: cluster
    rbac.deckhouse.io/aggregate-to-system-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
rules: []
---
# built-in, but not a role that collects a lineage: a compatibility alias of an old subsystem role
# selects the old lineage for the bindings to the old name, and the built-in modules no longer carry it
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:networking:viewer
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/deprecated: "true"
  annotations:
    rbac.deckhouse.io/deprecated-replaced-by: d8:subsystem:network:viewer
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-network-as: viewer
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-networking-as: viewer
      matchExpressions:
        - key: rbac.deckhouse.io/kind
          operator: NotIn
          values: ["use"]
rules: []
---
# built-in: a capability carries the lineage it is collected into and selects nothing
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:system-capability:deckhouse:edit
  labels:
    heritage: deckhouse
    module: deckhouse
    rbac.deckhouse.io/kind: capability
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/aggregate-to-cluster-as: manager
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
# counted: the selectors require kind: manage (the role of the FAQ example)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/aggregate-to-all-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
# counted: no kind label of its own, the selector requires kind: use
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:selectors-only:manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: use
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
rules: []
---
# counted: the name does not follow the custom:* convention
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: platform-operators
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-all-as: viewer
rules: []
---
# counted: a matchExpressions selector that accepts kind: manage
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:expressions:viewer
aggregationRule:
  clusterRoleSelectors:
    - matchExpressions:
        - key: rbac.deckhouse.io/kind
          operator: In
          values: ["manage", "custom-capability"]
        - key: rbac.deckhouse.io/aggregate-to-deckhouse-as
          operator: Exists
rules: []
---
# NOT counted: a capability, rules with the kind: use label and no aggregationRule
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
# NOT counted: a frozen role, the labels are kept, aggregationRule is removed
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:frozen:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-all-as: manager
rules:
  - apiGroups: ["deckhouse.io"]
    resources: ["moduleconfigs"]
    verbs: ["get", "list", "watch"]
---
# NOT counted: kind appears only with operators that do not require a legacy value
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:any-kind:viewer
aggregationRule:
  clusterRoleSelectors:
    - matchExpressions:
        - key: rbac.deckhouse.io/kind
          operator: Exists
        - key: rbac.deckhouse.io/aggregate-to-cluster-as
          operator: In
          values: ["viewer"]
    - matchExpressions:
        - key: rbac.deckhouse.io/kind
          operator: NotIn
          values: ["manage", "use"]
rules: []
---
# NOT counted: an In expression on the kind label that accepts the new-scheme values only
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:custom-capabilities-only
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
aggregationRule:
  clusterRoleSelectors:
    - matchExpressions:
        - key: rbac.deckhouse.io/kind
          operator: In
          values: ["custom-role", "custom-capability"]
        - key: rbac.deckhouse.io/aggregate-to-namespace-as
          operator: In
          values: ["viewer"]
rules: []
---
# NOT counted: the manage and use values under a key other than the kind label
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:other-key:viewer
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        team: manage
    - matchExpressions:
        - key: team
          operator: In
          values: ["manage", "use"]
rules: []
---
# NOT counted: custom:* name, but unrelated to the RBACv2 role model
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:totally-unrelated
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get"]
---
# NOT counted: new-scheme custom role, by a collected lineage and by the lineage of its own subsystem
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/use-role: user
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-mycustom-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-system-as: viewer
rules: []
---
# COUNTED: the selectors are rewritten, but the role keeps use-role without the system or subsystem
# scope, so it gets no RoleBindings in the namespaces of the modules any more
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:rewritten:manager
  labels:
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/use-role: user
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
rules: []
---
# NOT counted: a built-in (heritage: deckhouse) role with legacy selectors
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:all:manager
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/use-role: user
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: all
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-system-as: manager
rules: []
---
# COUNTED: a new-scheme custom role that selects by the lineage of a subsystem the model no longer has
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:ops:manager
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: ops
    rbac.deckhouse.io/use-role: admin
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
# COUNTED: any name, no kind, the lineage of a subsystem a module has left
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: virtualization-operators
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-virtualization-as: user
rules: []
---
# COUNTED: an Exists expression on the aggregation label of the old scheme
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:all-levels
aggregationRule:
  clusterRoleSelectors:
    - matchExpressions:
        - key: rbac.deckhouse.io/aggregate-to-all-as
          operator: Exists
rules: []
---
# COUNTED: the own subsystem of the role is a retired one, so its lineage is not a custom one
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:networking:auditor
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: networking
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-networking-as: viewer
rules: []
---
# NOT counted: the lineage of another custom subsystem, which no built-in role collects but the custom
# capability below carries
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:system:custom-subsystems
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: system
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-mycustom-as: manager
rules: []
---
# a custom capability of the FAQ example: it carries the lineage of its custom subsystem
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:subsystem-capability:mycustom:superresource:view
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/aggregate-to-mycustom-as: manager
rules:
  - apiGroups: ["mygroup.io"]
    resources: ["mysuperresources"]
    verbs: ["get", "list", "watch"]
---
# a custom capability that carries a retired lineage: a selector by it still counts
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:system-capability:networking:view
  labels:
    rbac.deckhouse.io/kind: custom-capability
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/aggregate-to-networking-as: viewer
rules:
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingressclasses"]
    verbs: ["get", "list", "watch"]
---
# COUNTED: a custom lineage that no built-in role collects and no custom capability carries
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:system:orphan-lineage
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: system
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-orphan-as: manager
rules: []
---
# COUNTED: only a custom role carries the lineage, and a role is not a capability
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:system:ladder-only
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: system
    rbac.deckhouse.io/aggregate-to-ladder-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-ladder-as: viewer
rules: []
---
# NOT counted: NotIn and DoesNotExist expressions on lineages no role collects require nothing
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:namespace:without-old-lineages
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: namespace
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-namespace-as: viewer
      matchExpressions:
        - key: rbac.deckhouse.io/aggregate-to-kubernetes-as
          operator: DoesNotExist
        - key: rbac.deckhouse.io/aggregate-to-all-as
          operator: NotIn
          values: ["viewer"]
rules: []
---
# NOT counted: selects by capability labels, which no lineage carries
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:cluster:node-operators
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: cluster
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/capability: system-capability.node-manager.view
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: node-manager
rules: []
`

// stateCollectedByBuiltinRoles has custom roles that select by lineages outside the subsystem list
// and the built-in roles that collect them: those of a module that ships a subsystem of its own and
// those of a subsystem the cluster still has.
const stateCollectedByBuiltinRoles = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:subsystem:virtualization:user
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/kind: role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: virtualization
aggregationRule:
  clusterRoleSelectors:
    - matchExpressions:
        - key: rbac.deckhouse.io/aggregate-to-virtualization-as
          operator: In
          values: ["user"]
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:subsystem:deckhouse:manager
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/kind: role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: deckhouse
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: virtualization-operators
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-virtualization-as: user
rules: []
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:ops:manager
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: ops
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
rules: []
`

// expectedLegacyCustomRoles are the roles of stateLegacyCustomRoles the hook reports, sorted.
var expectedLegacyCustomRoles = []string{
	"custom:all-levels",
	"custom:manage:expressions:viewer",
	"custom:manage:mycustom:manager",
	"custom:manage:rewritten:manager",
	"custom:manage:selectors-only:manager",
	"d8:custom:networking:auditor",
	"d8:custom:ops:manager",
	"d8:custom:system:ladder-only",
	"d8:custom:system:orphan-lineage",
	"platform-operators",
	"virtualization-operators",
}

var _ = Describe("User Authz hooks :: discovery legacy custom roles ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			requirements.RemoveValue(LegacyRBACv2CustomRolesValueKey)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("saves an empty list to the requirement value", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(LegacyRBACv2CustomRolesValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})

		It("emits no alert metrics, only the group expiration", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_custom_role",
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Cluster with a mix of legacy, frozen, new-scheme and unrelated roles", func() {
		BeforeEach(func() {
			requirements.RemoveValue(LegacyRBACv2CustomRolesValueKey)
			f.BindingContexts.Set(f.KubeStateSet(stateLegacyCustomRoles))
			f.RunHook()
		})

		It("saves only the roles whose selectors require a legacy kind or a lineage no built-in role collects and no custom capability carries, or whose use-role lacks the system or subsystem scope, sorted", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(LegacyRBACv2CustomRolesValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal(expectedLegacyCustomRoles))
		})

		It("emits one alert metric per legacy custom role", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(len(expectedLegacyCustomRoles) + 1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_custom_role",
				Action: operation.ActionExpireMetrics,
			}))
			for i, name := range expectedLegacyCustomRoles {
				Expect(m[i+1]).To(BeEquivalentTo(operation.MetricOperation{
					Name:   "d8_rbacv2_legacy_custom_role",
					Group:  "d8_rbacv2_legacy_custom_role",
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{"name": name},
				}))
			}
		})
	})

	Context("Cluster whose built-in roles collect the lineages the custom roles select", func() {
		BeforeEach(func() {
			requirements.RemoveValue(LegacyRBACv2CustomRolesValueKey)
			f.BindingContexts.Set(f.KubeStateSet(stateCollectedByBuiltinRoles))
			f.RunHook()
		})

		It("reports none of them", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(LegacyRBACv2CustomRolesValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})
	})
})
