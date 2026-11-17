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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: shared dex authenticator ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {"sharedDexAuthenticator": {}}}}`, "")

	const (
		passwordsPath = "userAuthn.internal.sharedDexAuthenticator.redisPasswords"
	)

	Context("While no instance has its Secret yet", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should generate a Redis password for each instance", func() {
			Expect(f).To(ExecuteSuccessfully())
			system := f.ValuesGet(passwordsPath + ".dex-authenticator-system").String()
			user := f.ValuesGet(passwordsPath + ".dex-authenticator").String()
			Expect(system).To(HaveLen(32))
			Expect(user).To(HaveLen(32))
			Expect(system).NotTo(Equal(user))
		})
	})

	Context("While one of the instances has its Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-system-redis
  namespace: d8-user-authn
data:
  password: QUJD # ABC
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-redis
  namespace: d8-user-authn
data: {}
`))
			f.RunHook()
		})

		It("Should keep the stored password and generate the missing one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(passwordsPath + ".dex-authenticator-system").String()).To(Equal("ABC"))
			Expect(f.ValuesGet(passwordsPath + ".dex-authenticator").String()).To(HaveLen(32))
		})
	})
})
