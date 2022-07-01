/*
Copyright 2022 Flant JSC

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

var _ = Describe("User Authz hooks :: handle cluster authorization rules ::", func() {
	f := HookExecutionConfigInit(`{"global": {}, "userAuthz": {"enableMultiTenancy": true, "internal": {"multitenancyCRDs": []}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "ClusterAuthorizationRule", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("CAR must be empty list", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(MatchJSON(`[]`))
		})
	})

	Context("Cluster with two CARs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCARs))
			f.RunHook()
		})

		It("CAR must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.crds").String()).To(MatchJSON(`[{"name":"car0","spec":{"accessLevel":"ClusterEditor", "subjects":[{"kind":"Group", "name":"NotEveryone"}]}},{"name":"car1","spec":{"accessLevel":"ClusterAdmin", "subjects":[{"kind":"Group", "name":"Everyone"}]}}]`))
		})
	})

	Context("Cluster with multitenancy rule", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultNS + multitenancyRule))
			f.RunHook()
		})

		It("CAR must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.multitenancyCRDs").String()).To(MatchJSON(`[{"name":"admin","spec":{"accessLevel":"SuperAdmin","allowAccessToSystemNamespaces":true,"limitNamespaces":["review-1","default"],"subjects":[{"kind":"User","name":"user@flant.com"}]}}]`))
		})
	})

	Context("Cluster with multitenancy rule and simple rules", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(defaultNS + multitenancyRule + stateCARs))
			f.RunHook()
		})

		It("CAR must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.multitenancyCRDs").Array()).To(HaveLen(1))
			Expect(f.ValuesGet("userAuthz.internal.crds").Array()).To(HaveLen(2))
		})
	})
})

const (
	stateCARs = `
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

	defaultNS = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: review-1
---
apiVersion: v1
kind: Namespace
metadata:
  name: default
`

	multitenancyRule = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: user@flant.com
  accessLevel: SuperAdmin
  allowAccessToSystemNamespaces: true
  limitNamespaces:
    - review-.*
`
)
