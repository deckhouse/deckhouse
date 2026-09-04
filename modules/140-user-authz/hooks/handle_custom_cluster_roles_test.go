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

		It("userAuthz.internal.customClusterRoles must be dicts of empty arrays", func() {
			ccrExpectation := `
			{
			  "user":[],
			  "privilegedUser":[],
			  "editor":[],
			  "admin":[],
			  "clusterEditor":[],
			  "clusterAdmin":[]
			}`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles").String()).To(MatchJSON(ccrExpectation))
		})
	})

	Context("Cluster with pile of Custom ClusterRoles", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCustomClusterRoles))
			f.RunHook()
		})

		It("Custom Roles and ClusterRoles must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			// ccr-wrong-label is annotated User: the annotation, not the (wrong) label, decides.
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.user").AsStringSlice()).Should(ConsistOf("ccr0", "ccr-wrong-label"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.privilegedUser").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr-wrong-label"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.editor").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr-wrong-label"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.admin").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr3", "ccr-wrong-label"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.clusterEditor").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr3", "ccr4", "ccr-wrong-label"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.clusterAdmin").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr3", "ccr4", "ccr5", "ccr-wrong-label"))
		})

		It("Roles with a stale or missing label are excluded from values", func() {
			Expect(f).To(ExecuteSuccessfully())
			for _, level := range []string{"user", "privilegedUser", "editor", "admin", "clusterEditor", "clusterAdmin"} {
				Expect(f.ValuesGet("userAuthz.internal.customClusterRoles." + level).AsStringSlice()).ShouldNot(ContainElement("ccr-stale-label"))
				Expect(f.ValuesGet("userAuthz.internal.customClusterRoles." + level).AsStringSlice()).ShouldNot(ContainElement("ccr-without-annotation0"))
			}
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.user").AsStringSlice()).Should(ContainElement("ccr-wrong-label"))
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
	})
})
