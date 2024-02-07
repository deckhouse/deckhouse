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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/hooks/generate_password"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: generate_password ", func() {
	var (
		hook = generate_password.NewBasicAuthPlainHook(generatePasswordSettings)

		testPassword    = generate_password.GeneratePassword()
		testPasswordB64 = base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf("admin:{PLAIN}%s", testPassword),
		))

		// Secret with password.
		authSecretManifest = `
---
apiVersion: v1
kind: Secret
metadata:
  name: ` + authSecretName + `
  namespace: ` + authSecretNS + `
data:
  auth: ` + testPasswordB64 + "\n"
	)

	f := HookExecutionConfigInit(`{"prometheus": {"internal": {"auth": {}}}}`, `{"prometheus":{}}`)

	Context("giving no Secret", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should generate new password", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(hook.PasswordInternalKey()).String()).ShouldNot(BeEmpty())
		})
	})

	Context("giving external auth configuration", func() {
		BeforeEach(func() {
			f.KubeStateSet(authSecretManifest)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml(hook.ExternalAuthKey(), []byte(`{"authURL": "test"}`))
			f.RunHook()
		})
		It("should delete password from values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(hook.PasswordInternalKey()).Exists()).Should(BeFalse(), "should delete internal value")
		})
	})

	Context("giving password in Secret", func() {
		BeforeEach(func() {
			f.KubeStateSet(authSecretManifest)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should set password value from Secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(hook.PasswordInternalKey()).String()).Should(BeEquivalentTo(testPassword))
		})

		Context("giving Secret is deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("should generate new password value", func() {
				Expect(f).To(ExecuteSuccessfully())
				pass := f.ValuesGet(hook.PasswordInternalKey()).String()
				Expect(pass).ShouldNot(BeEquivalentTo(testPassword))
				Expect(pass).ShouldNot(BeEmpty())
			})
		})
	})
})
