/*
Copyright 2023 Flant JSC

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
	stateAuthRules = `
---
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: ar0
  namespace: test
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: NotEveryone
---
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: ar1
  namespace: test
spec:
  accessLevel: Admin
  subjects:
  - kind: Group
    name: Everyone
`
)

var _ = Describe("User Authz hooks :: handle authorization rules ::", func() {
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "AuthorizationRule", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("CAR must be empty list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.authRuleCrds").String()).To(MatchJSON(`[]`))
		})
	})

	Context("Cluster with two ARs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateAuthRules))
			f.RunHook()
		})

		It("ARs must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.authRuleCrds").String()).To(MatchJSON(`[{"name":"ar0","namespace":"test","spec":{"accessLevel":"User", "allowScale": false, "portForwarding": false, "subjects":[{"kind":"Group", "name":"NotEveryone"}]}},{"name":"ar1","namespace":"test","spec":{"accessLevel":"Admin", "allowScale": false, "portForwarding": false, "subjects":[{"kind":"Group", "name":"Everyone"}]}}]`))
		})
	})
})
