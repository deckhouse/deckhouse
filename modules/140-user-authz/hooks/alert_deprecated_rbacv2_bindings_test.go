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
	// CRB to a deprecated per-module CAPABILITY — no alias, must be flagged as not-aliased.
	crbDeprecatedManageCap = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: legacy-modcap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:module:prometheus:view
subjects:
- kind: Group
  name: ops
  apiGroup: rbac.authorization.k8s.io
`
	// RoleBinding to a deprecated namespace CAPABILITY — no alias, must be flagged as not-aliased.
	rbDeprecatedUseCap = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: legacy-nscap
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:capability:kubernetes:view_secrets
subjects:
- kind: User
  name: alice@example.com
  apiGroup: rbac.authorization.k8s.io
`

	// The alias state-snapshotter keeps for the old name of its backup agent role, and a partner's
	// binding to it.
	crModuleAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:use:capability:module:state-snapshotter:backup_agent
  labels:
    heritage: deckhouse
    module: state-snapshotter
    rbac.deckhouse.io/deprecated: "true"
  annotations:
    rbac.deckhouse.io/deprecated-replaced-by: d8:state-snapshotter:backup-agent
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      state-snapshotter.deckhouse.io/aggregate-to-backup-agent: "true"
`
	rbModuleAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: backup-agent
  namespace: backup
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:capability:module:state-snapshotter:backup_agent
subjects:
- kind: ServiceAccount
  name: agent
  namespace: backup
`
	// A module that renamed a role already on the 1.78 naming (d8:<module>:<name>) and kept the old
	// name as an alias, a binding to that alias, and a binding to a current role of a module.
	crModuleRenamedAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:state-snapshotter:backup-agent
  labels:
    heritage: deckhouse
    module: state-snapshotter
    rbac.deckhouse.io/deprecated: "true"
  annotations:
    rbac.deckhouse.io/deprecated-replaced-by: d8:state-snapshotter:snapshot-agent
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      state-snapshotter.deckhouse.io/aggregate-to-snapshot-agent: "true"
`
	crbModuleRenamedAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: backup-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:state-snapshotter:backup-agent
subjects:
- kind: ServiceAccount
  name: agent
  namespace: backup
`
	crbCurrentModuleRole = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:node-manager:caps-controller-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:node-manager:caps-controller-manager
subjects:
- kind: ServiceAccount
  name: caps-controller-manager
  namespace: d8-cloud-instance-manager
`
	// Aliases a module labelled deprecated but left without the replaced-by annotation: one under a
	// legacy name, one under a 1.78 name.
	crModuleAliasNoReplacement = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:use:capability:module:state-snapshotter:backup_agent
  labels:
    heritage: deckhouse
    module: state-snapshotter
    rbac.deckhouse.io/deprecated: "true"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:state-snapshotter:backup-agent
  labels:
    heritage: deckhouse
    module: state-snapshotter
    rbac.deckhouse.io/deprecated: "true"
`
	// The alias a module that moved into the cluster subsystem keeps for its old role name, which is
	// in the legacy d8:manage: family, and a binding to it. The alias selects the capabilities of
	// the module only, so it grants less than the role it is replaced by.
	crModuleManageAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:virtualization:manager
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/deprecated: "true"
  annotations:
    rbac.deckhouse.io/deprecated-replaced-by: d8:subsystem:cluster:manager
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      module: virtualization
      rbac.deckhouse.io/kind: capability
      rbac.deckhouse.io/scope: system
      rbac.deckhouse.io/aggregate-to-cluster-as: manager
`
	crModuleManageAliasNoReplacement = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:virtualization:manager
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/deprecated: "true"
`
	crbModuleManageAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: virt-admins
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:virtualization:manager
subjects:
- kind: Group
  name: virt-admins
  apiGroup: rbac.authorization.k8s.io
`
	// A RoleBinding user-authz-controller keeps for crbDeprecatedManage in a module namespace, as
	// the previous release made it. It follows the ClusterRoleBinding, so it is not one to migrate.
	rbAutomatedProjection = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: d8:use:user:binding:legacy-observability
  namespace: d8-okmeter
  labels:
    heritage: deckhouse
    rbac.deckhouse.io/automated: "true"
  annotations:
    rbac.deckhouse.io/related-with: legacy-observability
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:role:user
subjects:
- kind: Group
  name: ops
  apiGroup: rbac.authorization.k8s.io
`
	// An alias of the platform: user-authz's own, labelled deprecated.
	crPlatformAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:use:role:viewer
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/deprecated: "true"
  annotations:
    rbac.deckhouse.io/deprecated-replaced-by: d8:namespace:viewer
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.deckhouse.io/aggregate-to-namespace-as: viewer
`
	// The alias of the platform that crbDeprecatedManage refers to.
	crPlatformManageAlias = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:observability:manager
  labels:
    heritage: deckhouse
    module: user-authz
    rbac.deckhouse.io/deprecated: "true"
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/use-role: user
  annotations:
    rbac.deckhouse.io/deprecated-replaced-by: d8:subsystem:observability:manager
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.deckhouse.io/aggregate-to-observability-as: manager
`
	// The roles and a capability of the previous scheme that modules released on their own still
	// ship under the legacy names, and bindings to them.
	crModuleRolesOfPreviousScheme = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:virtualization:manager
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/aggregate-to-all-as: manager
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.deckhouse.io/kind: manage
      rbac.deckhouse.io/aggregate-to-virtualization-as: manager
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:use:role:user:virtualization
  labels:
    heritage: deckhouse
    module: virtualization
    rbac.deckhouse.io/kind: use
    rbac.deckhouse.io/aggregate-to-all-as: user
aggregationRule:
  clusterRoleSelectors:
  - matchLabels:
      rbac.deckhouse.io/kind: use
      rbac.deckhouse.io/aggregate-to-virtualization-as: user
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:manage:permission:module:ingress-nginx:edit
  labels:
    heritage: deckhouse
    module: ingress-nginx
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: module
    rbac.deckhouse.io/aggregate-to-networking-as: manager
rules:
- apiGroups: ["deckhouse.io"]
  resources: ["ingressnginxcontrollers"]
  verbs: ["create", "update", "patch", "delete"]
`
	bindingsToModuleRolesOfPreviousScheme = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: virt-admins
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:virtualization:manager
subjects:
- kind: Group
  name: virt-admins
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: virt-users
  namespace: team-a
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:use:role:user:virtualization
subjects:
- kind: Group
  name: virt-users
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ingress-admins
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:manage:permission:module:ingress-nginx:edit
subjects:
- kind: Group
  name: ingress-admins
  apiGroup: rbac.authorization.k8s.io
`
)

var _ = Describe("User-authz hooks :: alert_deprecated_rbacv2_bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	Context("Bindings to deprecated roles and to a deprecated capability", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crPlatformAlias + crPlatformManageAlias + rbDeprecatedUse + crbDeprecatedManage + crbNewModel + rbOrdinary + crbDeprecatedManageCap))
			f.RunHook()
		})

		It("Saves the bindings to aliased names, sorted, as the requirement value", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			// The capability binding raises its alert but does not hold the release: it has no alias
			// whose removal would change anything for it.
			Expect(value).To(Equal([]string{
				"ClusterRoleBinding legacy-observability -> d8:manage:observability:manager",
				"RoleBinding team-a/legacy-viewer -> d8:use:role:viewer",
			}))
		})
	})

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

		It("Saves an empty list to the requirement value", func() {
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
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
			f.BindingContexts.Set(f.KubeStateSet(crPlatformManageAlias + crbDeprecatedManage + crbNewModel + rbOrdinary))
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
					"aliased":      "true",
					"replaced_by":  "",
				},
			}))
		})
	})

	Context("A ClusterRoleBinding to a deprecated manage role and the RoleBinding projected from it", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crPlatformManageAlias + crbDeprecatedManage + rbAutomatedProjection))
			f.RunHook()
		})

		It("Flags the ClusterRoleBinding only: the projection goes with it", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Labels["binding_name"]).To(Equal("legacy-observability"))
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal([]string{"ClusterRoleBinding legacy-observability -> d8:manage:observability:manager"}))
		})
	})

	Context("A RoleBinding to a deprecated use role", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crPlatformAlias + rbDeprecatedUse))
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
					"aliased":      "true",
					"replaced_by":  "",
				},
			}))
		})
	})

	Context("A ClusterRoleBinding to a deprecated per-module capability (no alias)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbDeprecatedManageCap))
			f.RunHook()
		})

		It("Flags it as not aliased (binding no longer grants access)", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   deprecatedRBACv2Metric,
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"binding_kind": "ClusterRoleBinding",
					"binding_name": "legacy-modcap",
					"namespace":    "",
					"role_name":    "d8:manage:permission:module:prometheus:view",
					"aliased":      "false",
					"replaced_by":  "",
				},
			}))
		})

		It("Does not hold the release: the requirement value stays empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})
	})

	Context("A binding to the alias a module keeps for a role of its own", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleAlias + rbModuleAlias + crPlatformAlias + rbDeprecatedUse))
			f.RunHook()
		})

		It("Flags it as aliased by the module, with the new name, whatever prefix the old name has", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(m[1:]).To(ConsistOf(
				operation.MetricOperation{
					Name:   deprecatedRBACv2Metric,
					Group:  deprecatedRBACv2Metric,
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{
						"binding_kind": "RoleBinding",
						"binding_name": "backup-agent",
						"namespace":    "backup",
						"role_name":    "d8:use:capability:module:state-snapshotter:backup_agent",
						"aliased":      "module",
						"replaced_by":  "d8:state-snapshotter:backup-agent",
					},
				},
				operation.MetricOperation{
					Name:   deprecatedRBACv2Metric,
					Group:  deprecatedRBACv2Metric,
					Action: operation.ActionGaugeSet,
					Value:  ptr.To(1.0),
					Labels: map[string]string{
						"binding_kind": "RoleBinding",
						"binding_name": "legacy-viewer",
						"namespace":    "team-a",
						"role_name":    "d8:use:role:viewer",
						"aliased":      "true",
						"replaced_by":  "",
					},
				},
			))
		})

		It("Holds the release for the alias of the platform only: the module removes its own", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal([]string{"RoleBinding team-a/legacy-viewer -> d8:use:role:viewer"}))
		})
	})

	Context("A RoleBinding to a deprecated namespace capability (no alias)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rbDeprecatedUseCap))
			f.RunHook()
		})

		It("Flags the use-capability RB as not aliased", func() {
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
					"binding_name": "legacy-nscap",
					"namespace":    "team-a",
					"role_name":    "d8:use:capability:kubernetes:view_secrets",
					"aliased":      "false",
					"replaced_by":  "",
				},
			}))
		})
	})
	Context("A binding to an alias a module keeps after renaming a role on the 1.78 naming", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleRenamedAlias + crbModuleRenamedAlias + crbCurrentModuleRole + crbNewModel))
			f.RunHook()
		})

		It("Flags the binding to the alias and nothing bound to a current role", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1]).To(Equal(operation.MetricOperation{
				Name:   deprecatedRBACv2Metric,
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"binding_kind": "ClusterRoleBinding",
					"binding_name": "backup-agent",
					"namespace":    "",
					"role_name":    "d8:state-snapshotter:backup-agent",
					"aliased":      "module",
					"replaced_by":  "d8:state-snapshotter:snapshot-agent",
				},
			}))
		})

		It("Does not hold the release", func() {
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})
	})

	Context("A binding to the alias a module keeps under a d8:manage: name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleManageAlias + crbModuleManageAlias + crPlatformManageAlias + crbDeprecatedManage))
			f.RunHook()
		})

		It("Flags it as aliased by the module, not by the platform, with the new name", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(m[1:]).To(ContainElement(operation.MetricOperation{
				Name:   deprecatedRBACv2Metric,
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"binding_kind": "ClusterRoleBinding",
					"binding_name": "virt-admins",
					"namespace":    "",
					"role_name":    "d8:manage:virtualization:manager",
					"aliased":      "module",
					"replaced_by":  "d8:subsystem:cluster:manager",
				},
			}))
		})

		It("Holds the release for the binding to the alias of the platform only", func() {
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal([]string{"ClusterRoleBinding legacy-observability -> d8:manage:observability:manager"}))
		})
	})

	Context("A binding to a d8:manage: alias of a module without the replaced-by annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleManageAliasNoReplacement + crbModuleManageAlias))
			f.RunHook()
		})

		It("Leaves it alone: the role is the module's, and no release of the platform removes it", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).To(BeEquivalentTo(operation.ActionExpireMetrics))
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})
	})

	Context("Bindings to aliases of a module without the replaced-by annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleAliasNoReplacement + rbModuleAlias + crbModuleRenamedAlias))
			f.RunHook()
		})

		It("Flags neither instead of an alert with no new name: both roles exist and grant access", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).To(BeEquivalentTo(operation.ActionExpireMetrics))
		})
	})

	Context("Bindings to the roles and a capability of the previous scheme that modules still ship", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleRolesOfPreviousScheme + bindingsToModuleRolesOfPreviousScheme + crPlatformManageAlias + crbDeprecatedManage))
			f.RunHook()
		})

		It("Flags the binding to the alias of the platform only: the roles of the modules keep working", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Labels).To(HaveKeyWithValue("binding_name", "legacy-observability"))
			Expect(m[1].Labels).To(HaveKeyWithValue("aliased", "true"))
		})

		It("Holds the release for the binding to the alias of the platform only", func() {
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(Equal([]string{"ClusterRoleBinding legacy-observability -> d8:manage:observability:manager"}))
		})
	})

	Context("Bindings to legacy names that no ClusterRole has", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crbDeprecatedManage + bindingsToModuleRolesOfPreviousScheme))
			f.RunHook()
		})

		It("Flags each as not aliased: the binding grants nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(5))
			for _, metric := range m[1:] {
				Expect(metric.Labels).To(HaveKeyWithValue("aliased", "false"), metric.Labels["binding_name"])
			}
		})

		It("Does not hold the release: no alias is removed under them", func() {
			value, exists := requirements.GetValue(DeprecatedRBACv2BindingsValueKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeEmpty())
		})
	})
})
