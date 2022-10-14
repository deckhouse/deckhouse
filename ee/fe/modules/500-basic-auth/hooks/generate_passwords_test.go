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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: basic-auth :: hooks :: generate_password", func() {

	const (
		locationsKey         = "basicAuth.locations"
		locationsInternalKey = "basicAuth.internal.locations"
		passwordKey          = "basicAuth.internal.locations.0.users.admin"
	)
	f := HookExecutionConfigInit(
		`{"basicAuth":{"internal":{}}}`,
		`{"basicAuth":{}}`,
	)

	Context("without secret", func() {
		Context("without locations", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})
			It("should generate new password", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(passwordKey).String()).Should(SatisfyAll(
					Not(BeEmpty()),
					HaveLen(generatedPasswdLength),
				))
			})
		})
		Context("with locations in config values", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ConfigValuesSetFromYaml(locationsKey, []byte(`[{"location": "/custom", "users": {"admin": "secret"}}]`))
				f.RunHook()
			})
			It("should put locations to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(passwordKey).String()).Should(BeEquivalentTo("secret"))
			})
		})
	})

	Context("with existing secret", func() {
		const generatedPasswd = "0123456789abcdefghij"
		adminPasswd := fmt.Sprintf("admin:{PLAIN}%s\n", generatedPasswd)
		adminPasswdB64 := base64.StdEncoding.EncodeToString([]byte(adminPasswd))
		htpasswdSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: htpasswd
  namespace: kube-basic-auth
data:
  htpasswd: |
    ` + adminPasswdB64

		customLocation := `
  # user:{PLAIN}password
  custom_location: |
    dXNlcjp7UExBSU59cGFzc3dvcmQK
`

		Context("without locations", func() {
			BeforeEach(func() {
				f.KubeStateSet(htpasswdSecret)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})
			It("should restore password from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(passwordKey).String()).Should(SatisfyAll(
					Not(BeEmpty()),
					Equal(generatedPasswd),
				))
			})
		})
		Context("with locations in config values", func() {
			BeforeEach(func() {
				f.KubeStateSet(htpasswdSecret)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ConfigValuesSetFromYaml(locationsKey, []byte(`[{"location": "/custom", "users": {"admin": "secret"}}]`))
				f.RunHook()
			})
			It("should put locations to values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(passwordKey).String()).Should(BeEquivalentTo("secret"))
			})
		})
		Context("no config values, custom locations in Secret", func() {
			BeforeEach(func() {
				f.KubeStateSet(htpasswdSecret + customLocation)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})
			It("should generate default location", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(passwordKey).String()).Should(SatisfyAll(
					Not(BeEmpty()),
					Not(Equal(generatedPasswd)),
					HaveLen(generatedPasswdLength),
				))
			})
		})
	})

})
