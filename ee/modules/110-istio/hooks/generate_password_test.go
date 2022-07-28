/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

var _ = Describe("Modules :: istio :: hooks :: generate_password ", func() {
	var (
		hook = generate_password.NewBasicAuthPlainHook(moduleValuesKey, authSecretNS, authSecretName)

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

	f := HookExecutionConfigInit(
		`{"istio":{"internal":{"auth": {}}}}`,
		`{"istio":{}}`,
	)

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
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSetFromYaml(hook.ExternalAuthKey(), []byte(`{"authURL": "test"}`))
			f.ValuesSet(hook.PasswordInternalKey(), []byte(`password`))
			f.RunHook()
		})
		It("should clean password from values", func() {
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

		Context("giving external auth configuration", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ValuesSetFromYaml(hook.ExternalAuthKey(), []byte(`{"authURL": "test"}`))
				f.RunHook()
			})
			It("should clean password from values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(hook.PasswordInternalKey()).Exists()).Should(BeFalse(), "should delete internal value")
			})
		})

	})

})
