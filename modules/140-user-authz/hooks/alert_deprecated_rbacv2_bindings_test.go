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
	// An alias of the platform: user-authz's own, told apart by name.
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
)

var _ = Describe("User-authz hooks :: alert_deprecated_rbacv2_bindings ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)

	Context("Bindings to deprecated roles and to a deprecated capability", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rbDeprecatedUse + crbDeprecatedManage + crbNewModel + rbOrdinary + crbDeprecatedManageCap))
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
					"aliased":      "true",
					"replaced_by":  "",
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

	Context("Bindings to aliases of a module without the replaced-by annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(crModuleAliasNoReplacement + rbModuleAlias + crbModuleRenamedAlias))
			f.RunHook()
		})

		It("Keeps the classification of the name instead of an alert with no new name", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2), "the binding to the 1.78 name is not flagged at all")
			Expect(m[1]).To(Equal(operation.MetricOperation{
				Name:   deprecatedRBACv2Metric,
				Group:  deprecatedRBACv2Metric,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"binding_kind": "RoleBinding",
					"binding_name": "backup-agent",
					"namespace":    "backup",
					"role_name":    "d8:use:capability:module:state-snapshotter:backup_agent",
					"aliased":      "false",
					"replaced_by":  "",
				},
			}))
		})
	})
})
