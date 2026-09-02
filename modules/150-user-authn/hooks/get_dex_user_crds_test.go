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
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: get dex user crds ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "User", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON("[]"))
		})
	})

	Context("Cluster with a User object", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
  ttl: 30m
`))
			f.RunHook()
		})
		It("Should fill internal values with the user name only", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[{"name": "admin"}]`))
		})
	})

	Context("Cluster with no User objects after they were present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
`))
			f.RunHook()
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should delete entry from internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON("[]"))
		})
	})

	Context("Cluster with a User whose email changed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: adminNext@example.com
  password: password
`))
			f.RunHook()
		})
		It("Should keep the user name in internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchJSON(`
[{"name": "admin"}]`))
		})
	})

	Context("Cluster with User objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  password: password
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: user
spec:
  email: user@example.com
  password: passwordNext
`))
			f.RunHook()
		})
		It("Should list user names without hashes or lock snapshots", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).To(MatchUnorderedJSON(`
[
  {"name": "admin"},
  {"name": "user"}
]`))
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).ToNot(ContainSubstring("password"))
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).ToNot(ContainSubstring("hash"))
			Expect(f.ValuesGet("userAuthn.internal.dexUsersCRDs").String()).ToNot(ContainSubstring("lock"))
		})
	})
})
