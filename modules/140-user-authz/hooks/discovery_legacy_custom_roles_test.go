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
        - key: rbac.deckhouse.io/aggregate-to-security-as
          operator: In
          values: ["viewer"]
    - matchExpressions:
        - key: rbac.deckhouse.io/kind
          operator: NotIn
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
# COUNTED: new-scheme custom role, by the lineage of a subsystem the new subsystem list replaces
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:subsystem:mycustom:manager
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
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
        rbac.deckhouse.io/aggregate-to-all-as: manager
rules: []
---
# NOT counted: new-scheme custom role, by lineages of the new role model and of its own subsystem
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:mycustom:manager
  labels:
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-cluster-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-mycustom-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
---
# NOT counted: an old-scheme role that selects the lineage of its own subsystem without the kind label
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:own-subsystem:manager
  labels:
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/subsystem: custom
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-custom-as: manager
rules: []
---
# COUNTED: any name, no kind, the lineage of a subsystem a module leaves for one of the new ones
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
# a built-in capability of the module that leaves the lineage: it carries the label until the update
# and does not keep the selector above working, so it is not a carrier
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:permission:subsystem:virtualization:manage_resources
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-virtualization-as: user
rules:
  - apiGroups: ["virtualization.deckhouse.io"]
    resources: ["virtualmachineclasses"]
    verbs: ["get", "list", "watch"]
---
# NOT counted: the lineage of another custom subsystem, which the new role model does not collect but
# the custom capability below carries
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
# COUNTED: a custom lineage that the new role model does not collect and no custom capability carries
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
# COUNTED: an Exists expression on the aggregation label of the experimental role model
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
# NOT counted: NotIn and DoesNotExist expressions on retired lineages require nothing
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
rules: []
---
# COUNTED for use-role only: the selector keeps its capabilities, but the update deletes the
# RoleBindings Deckhouse creates in the namespaces of the modules for the kind: manage and use-role
# labels
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:security-auditors:manager
  labels:
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/use-role: user
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-security-as: manager
rules: []
---
# NOT counted: use-role without kind: manage, Deckhouse creates no RoleBindings for it
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:use-role-without-kind:manager
  labels:
    rbac.deckhouse.io/use-role: user
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-security-as: manager
rules: []
---
# NOT counted: kind: manage without use-role
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:no-use-role:manager
  labels:
    rbac.deckhouse.io/kind: manage
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-storage-as: manager
rules: []
`

// expectedLegacyCustomRoles are the roles of stateLegacyCustomRoles the hook reports, sorted.
var expectedLegacyCustomRoles = []string{
	"custom:all-levels",
	"custom:manage:expressions:viewer",
	"custom:manage:mycustom:manager",
	"custom:manage:security-auditors:manager",
	"custom:manage:selectors-only:manager",
	"d8:custom:networking:auditor",
	"d8:custom:subsystem:mycustom:manager",
	"d8:custom:system:ladder-only",
	"d8:custom:system:orphan-lineage",
	"platform-operators",
	"virtualization-operators",
}

// expectedUseRoleOnlyRoles are the roles of expectedLegacyCustomRoles counted only for their kind:
// manage and use-role labels. custom:manage:mycustom:manager has the labels too, but its selectors
// count first.
var expectedUseRoleOnlyRoles = map[string]struct{}{
	"custom:manage:security-auditors:manager": {},
}

var _ = Describe("User Authz hooks :: discovery legacy custom roles ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	It("value key matches the contract with modules/140-user-authz/requirements", func() {
		// The requirements package duplicates this literal (module requirements packages stay
		// import-free of the hooks package); this assertion and its counterpart in
		// requirements/check_test.go pin both copies to the same string.
		Expect(LegacyRBACv2CustomRolesValueKey).To(Equal("userAuthz:legacyRBACv2CustomRoles"))
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			requirements.RemoveValue("userAuthz:legacyRBACv2CustomRoles")
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
			requirements.RemoveValue("userAuthz:legacyRBACv2CustomRoles")
			f.BindingContexts.Set(f.KubeStateSet(stateLegacyCustomRoles))
			f.RunHook()
		})

		It("saves only the roles whose selectors require a legacy kind or a lineage the new role model does not collect and no custom capability carries, or that carry the manage kind and use-role, sorted", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(LegacyRBACv2CustomRolesValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal(expectedLegacyCustomRoles))
		})

		It("emits one alert metric per legacy custom role, with the reason it is counted for", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(len(expectedLegacyCustomRoles) + 1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_rbacv2_legacy_custom_role",
				Action: operation.ActionExpireMetrics,
			}))
			for i, name := range expectedLegacyCustomRoles {
				reason := "selectors"
				if _, ok := expectedUseRoleOnlyRoles[name]; ok {
					reason = "use-role"
				}
				Expect(m[i+1]).To(BeEquivalentTo(operation.MetricOperation{
					Name:   "d8_rbacv2_legacy_custom_role",
					Group:  "d8_rbacv2_legacy_custom_role",
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{"name": name, "reason": reason},
				}))
			}
		})
	})
})
