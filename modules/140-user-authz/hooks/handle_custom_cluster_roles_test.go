/*
Copyright 2021 Flant JSC

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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	stateCustomClusterRoles = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-without-annotation0
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr0
  annotations:
    user-authz.deckhouse.io/access-level: User
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr1
  annotations:
    user-authz.deckhouse.io/access-level: PrivilegedUser
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr2
  annotations:
    user-authz.deckhouse.io/access-level: Editor
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr3
  annotations:
    user-authz.deckhouse.io/access-level: Admin
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr4
  annotations:
    user-authz.deckhouse.io/access-level: ClusterEditor
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr5
  annotations:
    user-authz.deckhouse.io/access-level: ClusterAdmin
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-stale-label
  labels:
    user-authz.deckhouse.io/access-level: Editor
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-wrong-label
  labels:
    user-authz.deckhouse.io/access-level: Admin
  annotations:
    user-authz.deckhouse.io/access-level: User
`
)

const accessLevelLabelPath = `metadata.labels.user-authz\.deckhouse\.io/access-level`

var _ = Describe("User Authz hooks :: handle custom cluster roles ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Runs without values and without patches", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with pile of Custom ClusterRoles", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCustomClusterRoles))
			f.RunHook()
		})

		It("Access-level label mirrors the annotation on every custom ClusterRole", func() {
			Expect(f).To(ExecuteSuccessfully())
			for name, level := range map[string]string{
				"ccr0": "User", "ccr1": "PrivilegedUser", "ccr2": "Editor",
				"ccr3": "Admin", "ccr4": "ClusterEditor", "ccr5": "ClusterAdmin",
				"ccr-wrong-label": "User",
			} {
				role := f.KubernetesGlobalResource("ClusterRole", name)
				Expect(role.Exists()).To(BeTrue(), name)
				Expect(role.Field(accessLevelLabelPath).String()).To(Equal(level), name)
			}
		})

		It("Stale label is removed and roles without the annotation stay untouched", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ClusterRole", "ccr-stale-label").Field(accessLevelLabelPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "ccr-without-annotation0").Field("metadata.labels").Exists()).To(BeFalse())
		})

		It("Patches exactly the roles whose label differs from the annotation", func() {
			Expect(f).To(ExecuteSuccessfully())
			// ccr0..ccr5 get the label, ccr-wrong-label is corrected, ccr-stale-label loses it.
			Expect(f.PatchCollector.Operations()).To(HaveLen(8))
		})

		It("Counts the custom roles per access level", func() {
			Expect(f).To(ExecuteSuccessfully())
			counts := map[string]float64{}
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == customClusterRolesMetric && m.Value != nil {
					counts[m.Labels["level"]] = *m.Value
				}
			}
			// ccr0 and ccr-wrong-label are User; one role per other level; the stale label counts nothing.
			Expect(counts).To(Equal(map[string]float64{
				"User": 2, "PrivilegedUser": 1, "Editor": 1, "Admin": 1, "ClusterEditor": 1, "ClusterAdmin": 1,
			}))
		})
	})

	Context("Cluster where every label already matches", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-labeled
  labels:
    user-authz.deckhouse.io/access-level: Editor
  annotations:
    user-authz.deckhouse.io/access-level: Editor
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-super-admin
  annotations:
    user-authz.deckhouse.io/access-level: SuperAdmin
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-unknown-level
  annotations:
    user-authz.deckhouse.io/access-level: Wrong
`))
			f.RunHook()
		})

		It("Issues no patch: a matching label is left alone and unknown levels get no label", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(BeEmpty())
			Expect(f.KubernetesGlobalResource("ClusterRole", "ccr-super-admin").Field(accessLevelLabelPath).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "ccr-unknown-level").Field(accessLevelLabelPath).Exists()).To(BeFalse())
		})
	})

	Context("Aggregated roles that did or did not collect their custom roles", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-user
  annotations:
    user-authz.deckhouse.io/access-level: User
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ccr-editor
  annotations:
    user-authz.deckhouse.io/access-level: Editor
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["delete"]
---
# the User aggregate has collected its role
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: user-authz:user:custom
aggregationRule:
  clusterRoleSelectors:
  - matchLabels: {user-authz.deckhouse.io/access-level: User}
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
---
# the Editor aggregate exists but is empty although User and Editor roles exist
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: user-authz:editor:custom
aggregationRule:
  clusterRoleSelectors:
  - matchLabels: {user-authz.deckhouse.io/access-level: Editor}
---
# the Admin aggregate as the API server stores a freshly rendered one: rules is null, not absent
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: user-authz:admin:custom
aggregationRule:
  clusterRoleSelectors:
  - matchLabels: {user-authz.deckhouse.io/access-level: Admin}
rules: null
`))
			f.RunHook()
		})

		It("Flags the levels whose aggregated role is empty or absent while they have roles to aggregate", func() {
			Expect(f).To(ExecuteSuccessfully())
			missing := map[string]float64{}
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == customAggregationMissingMetric && m.Value != nil {
					missing[m.Labels["level"]] = *m.Value
				}
			}
			// User: aggregated and filled. PrivilegedUser: aggregate absent but a User role must be in it.
			// Editor: aggregate present but empty. Admin and above: aggregates absent, roles expected.
			Expect(missing).To(Equal(map[string]float64{
				"User": 0, "PrivilegedUser": 1, "Editor": 1, "Admin": 1, "ClusterEditor": 1, "ClusterAdmin": 1,
			}))
		})
	})
})
