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
	enabled := HookExecutionConfigInit(`{"userAuthn":{"internal": {"sharedDexAuthenticator": {"enabled": true}}}}`, "")

	const (
		enabledPath   = "userAuthn.internal.sharedDexAuthenticator.enabled"
		passwordsPath = "userAuthn.internal.sharedDexAuthenticator.redisPasswords"
	)

	Context("While the module does not render the instances", func() {
		BeforeEach(func() {
			// Nothing in a cluster turns the instances on: neither an object nor a stored password.
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: shared-dex-authenticator-preview
  namespace: d8-user-authn
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-authenticator-system-redis
  namespace: d8-user-authn
data:
  password: QUJD # ABC
`))
			f.RunHook()
		})

		It("Should leave the instances off and generate no password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(enabledPath).Bool()).To(BeFalse())
			Expect(f.ValuesGet(passwordsPath).Exists()).To(BeFalse())
		})
	})

	Context("While the module renders the instances", func() {
		BeforeEach(func() {
			enabled.BindingContexts.Set(enabled.KubeStateSet(""))
			enabled.RunHook()
		})

		It("Should generate a Redis password for each instance", func() {
			Expect(enabled).To(ExecuteSuccessfully())
			Expect(enabled.ValuesGet(enabledPath).Bool()).To(BeTrue())
			system := enabled.ValuesGet(passwordsPath + ".dex-authenticator-system").String()
			user := enabled.ValuesGet(passwordsPath + ".dex-authenticator").String()
			Expect(system).To(HaveLen(32))
			Expect(user).To(HaveLen(32))
			Expect(system).NotTo(Equal(user))
		})
	})

	Context("While the module renders the instances and one of them has its Secret", func() {
		BeforeEach(func() {
			enabled.BindingContexts.Set(enabled.KubeStateSet(`
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
			enabled.RunHook()
		})

		It("Should keep the stored password and generate the missing one", func() {
			Expect(enabled).To(ExecuteSuccessfully())
			Expect(enabled.ValuesGet(passwordsPath + ".dex-authenticator-system").String()).To(Equal("ABC"))
			Expect(enabled.ValuesGet(passwordsPath + ".dex-authenticator").String()).To(HaveLen(32))
		})
	})
})
