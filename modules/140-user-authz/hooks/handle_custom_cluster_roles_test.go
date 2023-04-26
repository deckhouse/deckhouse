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
`

	stateCustomRoles = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cr-without-annotation0
  namespace: test
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cr0
  namespace: test
  annotations:
    user-authz.deckhouse.io/access-level: User
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cr1
  namespace: test
  annotations:
    user-authz.deckhouse.io/access-level: PrivilegedUser
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cr2
  namespace: test
  annotations:
    user-authz.deckhouse.io/access-level: Editor
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cr3
  namespace: test
  annotations:
    user-authz.deckhouse.io/access-level: Admin
`
)

var _ = Describe("User Authz hooks :: handle custom cluster roles ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("userAuthz.internal.customClusterRoles and userAuthz.internal.authRuleCrds must be dicts of empty arrays", func() {
			ccrExpectation := `
			{
			  "user":[],
			  "privilegedUser":[],
			  "editor":[],
			  "admin":[],
			  "clusterEditor":[],
			  "clusterAdmin":[]
			}`
			crExpectation := `
			{
			  "user":[],
			  "privilegedUser":[],
			  "editor":[],
			  "admin":[]
			}`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles").String()).To(MatchJSON(ccrExpectation))
			Expect(f.ValuesGet("userAuthz.internal.customRoles").String()).To(MatchJSON(crExpectation))
		})
	})

	Context("Cluster with pile of Custom Roles and ClusterRoles", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCustomRoles + stateCustomClusterRoles))
			f.RunHook()
		})

		It("Custom Roles and ClusterRoles must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.user").AsStringSlice()).Should(ConsistOf("ccr0"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.privilegedUser").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.editor").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.admin").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr3"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.clusterEditor").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr3", "ccr4"))
			Expect(f.ValuesGet("userAuthz.internal.customClusterRoles.clusterAdmin").AsStringSlice()).Should(ConsistOf("ccr0", "ccr1", "ccr2", "ccr3", "ccr4", "ccr5"))

			Expect(f.ValuesGet("userAuthz.internal.customRoles.user").AsStringSlice()).Should(ConsistOf("cr0"))
			Expect(f.ValuesGet("userAuthz.internal.customRoles.privilegedUser").AsStringSlice()).Should(ConsistOf("cr0", "cr1"))
			Expect(f.ValuesGet("userAuthz.internal.customRoles.editor").AsStringSlice()).Should(ConsistOf("cr0", "cr1", "cr2"))
			Expect(f.ValuesGet("userAuthz.internal.customRoles.admin").AsStringSlice()).Should(ConsistOf("cr0", "cr1", "cr2", "cr3"))
		})
	})
})
