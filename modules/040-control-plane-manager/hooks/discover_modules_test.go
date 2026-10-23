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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: discover_modules ::", func() {
	const (
		authzConfigMap = `
---
apiVersion: v1
data:
  url: https://authz-webhook-only.url
  ca: authz-webhook-ca-only
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
`
		authnWebhookConfigMapAdded = `
apiVersion: v1
data:
  url: https://authn-webhook-only.url
  ca: authn-webhook-ca-only
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
`

		authnOIDCConfigMapAdded = `
apiVersion: v1
data:
  oidcIssuerURL: https://oids-issuer-only.url
  oidcIssuerAddress: 1.1.1.1
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
`
		authzAndAuthzFullConfigMapAdded = `
---
apiVersion: v1
data:
  url: https://authz-webhook.url
  ca: authz-webhook-ca
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
---
apiVersion: v1
data:
  oidcIssuerURL: test
  oidcIssuerAddress: 8.8.8.8
  url: https://authn-webhook.url
  ca: authn-webhook-ca
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
`

		authzWithRevocationConfigMap = `
---
apiVersion: v1
data:
  url: https://127.0.0.1:40443/
  revocationURL: https://127.0.0.1:40443/revocation
  ca: authz-webhook-ca
kind: ConfigMap
metadata:
  name: control-plane-configurator
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
`

		auditFullConfigMapAdded = `
---
apiVersion: v1
data:
  url: https://audit-webhook.url
  ca: audit-webhook-ca
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-runtime-audit-engine
  labels:
    control-plane-configurator: ""
`
	)
	const values = `
controlPlaneManager:
  internal:
    audit: {}
    authz: {}
  apiserver:
    authn: {}
    authz: {}
global:
  discovery:
    kubernetesCA: kubernetesCATest
`

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(values, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully, but no values should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
		})

		Context("Someone added configmap with control-plane-configurator label in d8-user-authz namespace", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(authzConfigMap))
				f.RunHook()
			})

			It("controlPlaneManager.authz values must be filled with data from ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("https://authz-webhook-only.url"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("authz-webhook-ca-only"))
			})

			It("controlPlaneManager.apiserver.authn values for must not set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").Exists()).ToNot(BeTrue())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookURL").Exists()).ToNot(BeTrue())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookCA").Exists()).ToNot(BeTrue())
			})
		})

		Context("Someone added configmap with control-plane-configurator label in d8-user-authn namespace", func() {
			Context("with webhook settings only", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(authnWebhookConfigMapAdded))
					f.RunHook()
				})

				It("controlPlaneManager.apiserver.authn values for webhook must be filled with data from ConfigMap", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookURL").String()).To(Equal("https://authn-webhook-only.url"))
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookCA").String()).To(Equal("authn-webhook-ca-only"))
				})

				It("controlPlaneManager.apiserver.authn values for oidc issuer must not be set", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").Exists()).ToNot(BeTrue())
				})

				It("controlPlaneManager.authz values must be not set", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
				})
			})

			Context("with oidc provider settings only", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(authnOIDCConfigMapAdded))
					f.RunHook()
				})

				It("controlPlaneManager.apiserver.authn values for oidc issuer must be filled with data from ConfigMap", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").String()).To(Equal("https://oids-issuer-only.url"))
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").String()).To(Equal("kubernetesCATest"))
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").String()).To(Equal("1.1.1.1"))
				})

				It("controlPlaneManager.apiserver.authn values for webhook must be not set", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookCA").Exists()).ToNot(BeTrue())
				})

				It("controlPlaneManager.authz values must be not set", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
				})
			})
		})
	})

	// The revocation check of user-authn is served by the user-authz webhook: its URL comes from the
	// ConfigMap of user-authz, together with the webhook. Every control-plane-manager Pod writes its
	// kubeconfig from 1.77.N on, and channel updates go through that release, so the check is added
	// right away, without waiting for a rollout.
	Context("The revocation check of user-authn", func() {
		const revocationURLPath = "controlPlaneManager.internal.authz.revocationWebhookURL"

		f := HookExecutionConfigInit(values, `{}`)

		expectURL := func(expected bool) {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(revocationURLPath).Exists()).To(Equal(expected))
			if expected {
				Expect(f.ValuesGet(revocationURLPath).String()).To(Equal("https://127.0.0.1:40443/revocation"))
			}
		}

		for _, tc := range []struct {
			name     string
			state    string
			expected bool
		}{
			{name: "when user-authz names the check", state: authzWithRevocationConfigMap, expected: true},
			{name: "when user-authz does not name the check", state: authzConfigMap},
			{name: "without the ConfigMap of user-authz", state: ""},
		} {
			Context(tc.name, func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(tc.state))
					f.RunHook()
				})

				It(fmt.Sprintf("must set the URL: %v", tc.expected), func() {
					expectURL(tc.expected)
				})
			})
		}

		Context("user-authz stops naming the check", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(authzWithRevocationConfigMap))
				f.RunHook()
				expectURL(true)
				f.BindingContexts.Set(f.KubeStateSet(authzConfigMap))
				f.RunHook()
			})

			It("drops it and keeps the webhook", func() {
				expectURL(false)
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("https://authz-webhook-only.url"))
			})
		})
	})

	// The revocation check is served by the user-authz webhook. An authorization webhook configured
	// by hand is some other authorizer, so the check must not be added next to it.
	Context("The authorization webhook is configured by hand", func() {
		const manualConfig = `{"controlPlaneManager":{"apiserver":{"authz":{"webhookURL":"https://manual-authz.url","webhookCA":"manual-ca"}}}}`
		const manualValues = `
controlPlaneManager:
  internal:
    audit: {}
    authz:
      revocationWebhookURL: https://127.0.0.1:40443/revocation
  apiserver:
    authn: {}
    authz:
      webhookURL: https://manual-authz.url
      webhookCA: manual-ca
global:
  discovery:
    kubernetesCA: kubernetesCATest
`
		f := HookExecutionConfigInit(manualValues, manualConfig)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(authzWithRevocationConfigMap))
			f.RunHook()
		})

		It("must not add the revocation check, even when user-authz names it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("https://manual-authz.url"))
			Expect(f.ValuesGet("controlPlaneManager.internal.authz.revocationWebhookURL").Exists()).To(BeFalse())
		})
	})

	Context("Secret d8-cloud-instance-manager-cloud-provider is in cluster", func() {
		f := HookExecutionConfigInit(values, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(authzConfigMap))
			f.RunHook()
		})

		It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("https://authz-webhook-only.url"))
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("authz-webhook-ca-only"))
		})

		Context("ConfigMap was added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(authzAndAuthzFullConfigMapAdded))
				f.RunHook()
			})

			It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("https://authz-webhook.url"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("authz-webhook-ca"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").String()).To(Equal("test"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").String()).To(Equal("kubernetesCATest"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").String()).To(Equal("8.8.8.8"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookURL").String()).To(Equal("https://authn-webhook.url"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookCA").String()).To(Equal("authn-webhook-ca"))
			})

			Context("ConfigMaps were deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})

				It("Hook must execute successfully, and all values should be unset", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerAddress").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.webhookCA").Exists()).ToNot(BeTrue())
				})

				Context("Audit ConfigMap was added", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSet(auditFullConfigMapAdded))
						f.RunHook()
					})

					It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
						Expect(f).To(ExecuteSuccessfully())
						Expect(f.ValuesGet("controlPlaneManager.internal.audit.webhookURL").String()).To(Equal("https://audit-webhook.url"))
						Expect(f.ValuesGet("controlPlaneManager.internal.audit.webhookCA").String()).To(Equal("audit-webhook-ca"))
					})

					Context("ConfigMaps were deleted", func() {
						BeforeEach(func() {
							f.BindingContexts.Set(f.KubeStateSet(""))
							f.RunHook()
						})

						It("Hook must execute successfully, and all values should be unset", func() {
							Expect(f).To(ExecuteSuccessfully())
							Expect(f.ValuesGet("controlPlaneManager.internal.audit.webhookURL").Exists()).To(BeFalse())
							Expect(f.ValuesGet("controlPlaneManager.internal.audit.webhookCA").Exists()).To(BeFalse())
						})
					})
				})
			})
		})
	})

	Context("enableBasicAuth discovery", func() {
		const (
			enableBasicAuthValues = `
controlPlaneManager:
  internal:
    authn: {}
  apiserver:
    authn: {}
    authz: {}
global:
  discovery:
    kubernetesCA: kubernetesCATest
`
			cmWithBasicAuth = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: control-plane-configurator
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
data:
  enableBasicAuth: "true"
`
			cmWithInvalidBasicAuth = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: control-plane-configurator
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
data:
  enableBasicAuth: "broken"
`
			cmWithoutBasicAuth = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: control-plane-configurator
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
data:
  oidcIssuerURL: https://example.com/
`
		)

		f := HookExecutionConfigInit(enableBasicAuthValues, `{}`)

		It("Removes enableBasicAuth when the setting disappears from the ConfigMap", func() {
			f.BindingContexts.Set(f.KubeStateSet(cmWithBasicAuth))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.enableBasicAuth").Bool()).To(BeTrue())

			f.BindingContexts.Set(f.KubeStateSet(cmWithoutBasicAuth))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.enableBasicAuth").Exists()).To(BeFalse())
		})

		It("Removes enableBasicAuth when the setting is not a boolean", func() {
			f.BindingContexts.Set(f.KubeStateSet(cmWithBasicAuth))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.enableBasicAuth").Bool()).To(BeTrue())

			f.BindingContexts.Set(f.KubeStateSet(cmWithInvalidBasicAuth))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.enableBasicAuth").Exists()).To(BeFalse())
		})
	})
})
