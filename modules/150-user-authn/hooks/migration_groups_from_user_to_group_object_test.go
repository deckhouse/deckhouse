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

var _ = Describe("User Authn hooks :: migration to Group object ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "User", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "Group", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("Cluster with User objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  groups:
  - Admins
  - Everyone
  password: password
---
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: user
spec:
  email: user@example.com
  groups:
  - Everyone
  password: passwordNext
`))
			f.RunHook()
		})
		It("Should synchronize objects and fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Group", "admins").Parse().Raw).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "Group",
  "metadata": {
    "creationTimestamp": null,
    "name": "admins"
  },
  "spec": {
    "members": [
      {
        "kind": "User",
        "name": "admin"
      }
    ],
    "name": "Admins"
  },
  "status": {}
}`))
			Expect(f.KubernetesGlobalResource("Group", "everyone").Parse().Raw).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1alpha1",
  "kind": "Group",
  "metadata": {
    "creationTimestamp": null,
    "name": "everyone"
  },
  "spec": {
    "members": [
      {
        "kind": "User",
        "name": "admin"
      },
      {
        "kind": "User",
        "name": "user"
      }
    ],
    "name": "Everyone"
  },
  "status": {}
}`))
		})
	})
})
