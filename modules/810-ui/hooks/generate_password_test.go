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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type appTestSettings struct {
	appName    string
	secretName string

	password          string
	generatedPassword string

	externalAuthValuesPath     string
	passwordInternalValuesPath string
}

func (a *appTestSettings) GeneratedSecret() string {
	return `
---
apiVersion: v1
kind: Secret
metadata:
  name: ` + a.secretName + `
  namespace: ` + upmeterNS + `
data:
  auth: ` + base64.StdEncoding.EncodeToString([]byte("admin:{PLAIN}"+a.generatedPassword)) + "\n"
}

func (a *appTestSettings) CustomSecret() string {
	return `
---
apiVersion: v1
kind: Secret
metadata:
  name: ` + a.secretName + `
  namespace: ` + upmeterNS + `
data:
  auth: ` + base64.StdEncoding.EncodeToString([]byte("admin:{PLAIN}"+a.password)) + "\n"
}

var _ = Describe("Modules :: upmeter :: hooks :: generate_password", func() {
	testSettings := make(map[string]*appTestSettings)
	for secretName, appName := range upmeterApps {
		settings := &appTestSettings{
			secretName:                 secretName,
			appName:                    appName,
			password:                   fmt.Sprintf("t3stPassw0rd-%s", appName),
			generatedPassword:          GeneratePassword(),
			externalAuthValuesPath:     fmt.Sprintf(externalAuthValuesTmpl, appName),
			passwordInternalValuesPath: fmt.Sprintf(passwordInternalValuesTmpl, appName),
		}

		testSettings[appName] = settings
	}

	for appName, settings := range testSettings {
		Context(appName, func() {

			// Initialize internal.auth object for values patch to work.
			f := HookExecutionConfigInit(
				`{"global":{}, "upmeter": {"internal": {"auth": {"status": {}, "webui": {}}}} }`,
				`{"upmeter":{}}`,
			)

			Context("giving no Secret", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})

				It("should generate new password", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet(settings.passwordInternalValuesPath).String()).ShouldNot(BeEmpty())
				})
			})

			Context("giving external auth configuration", func() {
				BeforeEach(func() {
					f.KubeStateSet("")
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.ValuesSetFromYaml(settings.externalAuthValuesPath, []byte(`{"authURL": "test"}`))
					f.ValuesSet(settings.passwordInternalValuesPath, []byte(`password`))
					f.RunHook()
				})
				It("should clean password from values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet(settings.passwordInternalValuesPath).Exists()).Should(BeFalse(), "should delete internal value")
				})
			})

			Context("giving password in Secret", func() {
				BeforeEach(func() {
					f.KubeStateSet(settings.GeneratedSecret())
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())
					f.RunHook()
				})
				It("should set password value from Secret", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet(settings.passwordInternalValuesPath).String()).Should(BeEquivalentTo(settings.generatedPassword))
				})

				Context("giving Secret is deleted", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSet(""))
						f.RunHook()
					})
					It("should generate new password value", func() {
						Expect(f).To(ExecuteSuccessfully())
						pass := f.ValuesGet(settings.passwordInternalValuesPath).String()
						Expect(pass).ShouldNot(BeEquivalentTo(settings.generatedPassword))
						Expect(pass).ShouldNot(BeEmpty())
					})
				})

				Context("giving external auth configuration", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.GenerateBeforeHelmContext())
						f.ValuesSetFromYaml(settings.externalAuthValuesPath, []byte(`{"authURL": "test"}`))
						f.RunHook()
					})
					It("should clean password from values", func() {
						Expect(f).To(ExecuteSuccessfully())
						Expect(f.ValuesGet(settings.passwordInternalValuesPath).Exists()).Should(BeFalse(), "should delete internal value")
					})
				})

			})

		})
	}

	Context("all apps", func() {
		f := HookExecutionConfigInit(
			`{"upmeter": {"internal": {"auth": {"status": {}, "webui": {}}}} }`,
			`{"upmeter":{}}`,
		)

		Context("giving no Secret", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("should generate new password", func() {
				Expect(f).To(ExecuteSuccessfully())

				for appName, settings := range testSettings {
					Expect(f.ValuesGet(settings.passwordInternalValuesPath).String()).ShouldNot(BeEmpty(), "Should generate password for '%s'", appName)
				}
			})
		})

		Context("giving external auth configuration", func() {
			BeforeEach(func() {
				f.KubeStateSet("")
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())

				extAuth := []byte(`{"authURL": "test"}`)

				for _, settings := range testSettings {
					f.ValuesSetFromYaml(settings.externalAuthValuesPath, extAuth)
					f.ValuesSet(settings.passwordInternalValuesPath, []byte(`password`))
				}

				f.RunHook()
			})
			It("should clean password from values", func() {
				Expect(f).To(ExecuteSuccessfully())
				for appName, settings := range testSettings {
					Expect(f.ValuesGet(settings.passwordInternalValuesPath).Exists()).Should(BeFalse(), "should delete internal value for '%s'", appName)
				}
			})
		})

		Context("giving password in Secret", func() {
			BeforeEach(func() {
				secrets := ""
				for _, settings := range testSettings {
					secrets += settings.GeneratedSecret()
				}
				f.KubeStateSet(secrets)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})
			It("should restore generated passwords", func() {
				Expect(f).To(ExecuteSuccessfully())
				for _, settings := range testSettings {
					Expect(f.ValuesGet(settings.passwordInternalValuesPath).String()).Should(BeEquivalentTo(settings.generatedPassword))
				}
			})

			Context("giving Secret is deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})
				It("should generate new passwords", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f).To(ExecuteSuccessfully())
					for _, settings := range testSettings {
						pass := f.ValuesGet(settings.passwordInternalValuesPath).String()
						Expect(pass).ShouldNot(BeEquivalentTo(settings.generatedPassword))
						Expect(pass).ShouldNot(BeEmpty())
					}
				})
			})

			Context("giving external auth configuration", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.GenerateBeforeHelmContext())

					extAuth := []byte(`{"authURL": "test"}`)

					for _, settings := range testSettings {
						f.ValuesSetFromYaml(settings.externalAuthValuesPath, extAuth)
						f.ValuesSet(settings.passwordInternalValuesPath, []byte(`password`))
					}

					f.RunHook()
				})
				It("should clean password from values", func() {
					Expect(f).To(ExecuteSuccessfully())
					for appName, settings := range testSettings {
						Expect(f.ValuesGet(settings.passwordInternalValuesPath).Exists()).Should(BeFalse(), "should delete internal value for '%s'", appName)
					}
				})
			})
		})

	})
})
