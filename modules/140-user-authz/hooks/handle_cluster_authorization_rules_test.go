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
	stateClusterAuthRules = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: car0
spec:
  accessLevel: ClusterEditor
  subjects:
  - kind: Group
    name: NotEveryone
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: car1
spec:
  accessLevel: ClusterAdmin
  subjects:
  - kind: Group
    name: Everyone
`

	// The aggregated custom binding of car0 exists (rendered by the chart); car1 still has none.
	// The unlabeled binding proves that only bindings carrying the binding-kind label count.
	stateAggregatedBindings = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: user-authz:car0:cluster-editor:custom
  labels:
    user-authz.deckhouse.io/binding-kind: aggregated-custom
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: user-authz:cluster-editor:custom
subjects:
- kind: Group
  name: NotEveryone
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: user-authz:car1:cluster-admin:custom
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: user-authz:cluster-admin:custom
subjects:
- kind: Group
  name: Everyone
`

	stateClusterAuthRulesWithoutLevel = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: car-roles-only
spec:
  additionalRoles:
  - apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: view
  subjects:
  - kind: Group
    name: Viewers
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: car-super
spec:
  accessLevel: SuperAdmin
  subjects:
  - kind: Group
    name: Root
`
)

var _ = Describe("User Authz hooks :: handle cluster authorization rules ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "ClusterAuthorizationRule", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("CAR must be empty list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds").String()).To(MatchJSON(`[]`))
		})
	})

	Context("Cluster with two CARs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateClusterAuthRules))
			f.RunHook()
		})

		It("CARs must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds").String()).To(MatchJSON(`[{"name":"car0","legacyCustomRoleBindings":true,"spec":{"accessLevel":"ClusterEditor", "allowScale": false, "portForwarding": false, "subjects":[{"kind":"Group", "name":"NotEveryone"}]}},{"name":"car1","legacyCustomRoleBindings":true,"spec":{"accessLevel":"ClusterAdmin", "allowScale": false, "portForwarding": false, "subjects":[{"kind":"Group", "name":"Everyone"}]}}]`))
		})
	})

	Context("Cluster with two CARs and the aggregated binding of one of them", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateClusterAuthRules + stateAggregatedBindings))
			f.RunHook()
		})

		It("Only the CAR without an aggregated binding keeps the legacy per-role bindings", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.0.name").String()).To(Equal("car0"))
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.0.legacyCustomRoleBindings").Bool()).To(BeFalse())
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.1.name").String()).To(Equal("car1"))
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.1.legacyCustomRoleBindings").Bool()).To(BeTrue())
		})
	})

	Context("Cluster with CARs that never get custom-role bindings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateClusterAuthRulesWithoutLevel))
			f.RunHook()
		})

		It("Rules without accessLevel and SuperAdmin rules do not ask for legacy bindings", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.0.name").String()).To(Equal("car-roles-only"))
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.0.legacyCustomRoleBindings").Bool()).To(BeFalse())
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.1.name").String()).To(Equal("car-super"))
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds.1.legacyCustomRoleBindings").Bool()).To(BeFalse())
		})
	})
})
