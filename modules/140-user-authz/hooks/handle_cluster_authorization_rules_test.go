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
			Expect(f.ValuesGet("userAuthz.internal.clusterAuthRuleCrds").String()).To(MatchJSON(`[{"name":"car0","spec":{"accessLevel":"ClusterEditor", "allowScale": false, "portForwarding": false, "subjects":[{"kind":"Group", "name":"NotEveryone"}]}},{"name":"car1","spec":{"accessLevel":"ClusterAdmin", "allowScale": false, "portForwarding": false, "subjects":[{"kind":"Group", "name":"Everyone"}]}}]`))
		})
	})
})
