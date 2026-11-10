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

package template_tests

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: dex-config", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.21.1")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "test")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "testcert")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "testkey")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "testCA")

		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
	})

	Context("Without Users and Providers", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})

		It("Should create dex config with enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).To(BeEmpty())
			Expect(config).To(ContainSubstring("enablePasswordDB: true"))
		})

		It("Should render the Dex v2.46 configuration format", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			By("signing keys rotate every 6 hours through the signer, not the deprecated expiry.signingKeys")
			Expect(gjson.GetBytes(jsonConfig, "signer.type").String()).To(Equal("local"))
			Expect(gjson.GetBytes(jsonConfig, "signer.config.keysRotationPeriod").String()).To(Equal("6h"))
			Expect(gjson.GetBytes(jsonConfig, "expiry.signingKeys").Exists()).To(BeFalse())

			By("the client address is X-Real-IP from any peer: patch 245 needs trustedProxies unset")
			Expect(gjson.GetBytes(jsonConfig, "web.clientRemoteIP.header").String()).To(Equal("X-Real-IP"))
			Expect(gjson.GetBytes(jsonConfig, "web.clientRemoteIP.trustedProxies").Exists()).To(BeFalse())

			By("without 2FA there is no MFA, and the approval page is skipped")
			Expect(gjson.GetBytes(jsonConfig, "mfa").Exists()).To(BeFalse())
			Expect(gjson.GetBytes(jsonConfig, "twoFactorAuthn").Exists()).To(BeFalse())
			Expect(gjson.GetBytes(jsonConfig, "oauth2.skipApprovalScreen").Type).To(Equal(gjson.True))

			By("PKCE accepts only S256 and stays optional")
			Expect(gjson.GetBytes(jsonConfig, "oauth2.pkce.codeChallengeMethodsSupported").Value()).To(Equal([]any{"S256"}))
			Expect(gjson.GetBytes(jsonConfig, "oauth2.pkce.enforce").Exists()).To(BeFalse())

			By("browser sessions stay off")
			Expect(gjson.GetBytes(jsonConfig, "sessions").Exists()).To(BeFalse())

			By("patch 420: sign-in providers cannot assert the names the API server and peering reserve")
			Expect(gjson.GetBytes(jsonConfig, "reservedNamePrefixes").Value()).To(Equal([]any{"system:", "peering:"}))

			By("patch 425: the kubeconfig clients, whose secret users know, may not exchange tokens")
			Expect(gjson.GetBytes(jsonConfig, "tokenExchange.deniedClients").Value()).To(Equal([]any{"kubernetes", "kubeconfig-generator", "kubeconfig-publish-api"}))
		})
	})

	Context("With kubeconfigGenerator entries", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.kubeconfigGenerator", `[{id: "prod@a:b", masterURI: "https://1.2.3.4:6443", description: Prod}]`)
			hec.HelmRender()
		})

		It("Should deny the token exchange to every kubeconfig client", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)
			Expect(gjson.GetBytes(jsonConfig, "tokenExchange.deniedClients").Value()).To(Equal([]any{
				"kubernetes", "kubeconfig-generator", "kubeconfig-publish-api", "kubeconfig-generator-0", "kubeconfig-prod-at-a-b",
			}))
		})
	})

	Context("With 2FA", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.issuerName", "Deckhouse (Alpha)")
			hec.HelmRender()
		})

		It("Should render the TOTP authenticator of local users", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa").Raw).To(MatchJSON(`{
				"authenticators": [{
					"id": "totp",
					"type": "TOTP",
					"config": {"issuer": "Deckhouse (Alpha)"},
					"connectorTypes": ["local"]
				}],
				"defaultMFAChain": ["totp"]
			}`))
			Expect(gjson.GetBytes(jsonConfig, "twoFactorAuthn").Exists()).To(BeFalse())
		})

		It("Should skip the approval page and keep sessions off", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "oauth2.skipApprovalScreen").Type).To(Equal(gjson.True))
			Expect(gjson.GetBytes(jsonConfig, "sessions").Exists()).To(BeFalse())
		})
	})

	Context("With 2FA for the users of LDAP and Crowd providers", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"Local", "LDAP", "Crowd"})
			hec.HelmRender()
		})

		It("Should bind the TOTP authenticator to the connector types of Dex", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.connectorTypes").Value()).To(Equal([]any{"local", "ldap", "atlassian-crowd"}))
		})
	})

	Context("With 2FA for the LDAP providers only", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"LDAP"})
			hec.HelmRender()
		})

		It("Should leave the local users without the second factor", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.connectorTypes").Value()).To(Equal([]any{"ldap"}))
		})
	})

	// The webhooks compare each object with a snapshot of the others, and one apply of both may pass them: the render
	// refuses the combination too, on the settings after the conversion of their version.
	Context("With 2FA for a type whose provider has Basic Authentication", func() {
		for _, tc := range []struct {
			name      string
			types     []string
			provider  string
			wantError string
		}{
			{name: "LDAP covered", types: []string{"Local", "LDAP"}, provider: `
- id: corp
  type: LDAP
  ldap: {host: ldap.example.com, enableBasicAuth: true}`, wantError: `twoFactor asks the users of the LDAP providers for the second factor, which Basic Authentication to the Kubernetes API cannot give: turn enableBasicAuth off in the DexProvider "corp"`},
			{name: "Crowd covered", types: []string{"Crowd"}, provider: `
- id: crowd
  type: Crowd
  crowd: {baseURL: https://crowd.example.com, clientID: id, clientSecret: secret, enableBasicAuth: true}`, wantError: `twoFactor asks the users of the Crowd providers`},
			{name: "LDAP not covered", types: []string{"Local", "Crowd"}, provider: `
- id: corp
  type: LDAP
  ldap: {host: ldap.example.com, enableBasicAuth: true}`},
			{name: "LDAP covered without Basic Authentication", types: []string{"LDAP"}, provider: `
- id: corp
  type: LDAP
  ldap: {host: ldap.example.com, enableBasicAuth: false}`},
			{name: "OIDC with Basic Authentication", types: []string{"Local", "LDAP", "Crowd"}, provider: `
- id: idp
  type: OIDC
  oidc: {issuer: https://idp.example.com, clientID: id, clientSecret: secret, enableBasicAuth: true}`},
		} {
			Context(tc.name, func() {
				BeforeEach(func() {
					hec.ValuesSet("userAuthn.twoFactor.enabled", true)
					hec.ValuesSet("userAuthn.twoFactor.providerTypes", tc.types)
					hec.ValuesSetFromYaml("userAuthn.internal.providers", tc.provider)
					hec.HelmRender()
				})

				It("Should refuse the render only when the provider has Basic Authentication and its type is covered", func() {
					if tc.wantError == "" {
						Expect(hec.RenderError).ToNot(HaveOccurred())
						return
					}
					Expect(hec.RenderError).To(HaveOccurred())
					Expect(hec.RenderError.Error()).To(ContainSubstring(tc.wantError))
				})
			})
		}

		Context("with 2FA off", func() {
			BeforeEach(func() {
				hec.ValuesSet("userAuthn.twoFactor.enabled", false)
				hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"LDAP"})
				hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: corp
  type: LDAP
  ldap: {host: ldap.example.com, enableBasicAuth: true}`)
				hec.HelmRender()
			})

			It("Should render", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())
			})
		})
	})

	Context("With WebAuthn", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.methods", []string{"WebAuthn"})
			hec.ValuesSet("userAuthn.twoFactor.issuerName", "Deckhouse (Alpha)")
			hec.HelmRender()
		})

		It("Should render the WebAuthn authenticator, with no attestation and the domain of the issuer", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa").Raw).To(MatchJSON(`{
				"authenticators": [{
					"id": "webauthn",
					"type": "WebAuthn",
					"config": {"rpDisplayName": "Deckhouse (Alpha)", "attestationPreference": "none"},
					"connectorTypes": ["local"]
				}],
				"defaultMFAChain": ["webauthn"]
			}`))
		})
	})

	Context("With a relying party ID", func() {
		for _, tc := range []struct {
			name      string
			rpID      string
			wantError string
		}{
			{name: "the Dex host", rpID: "dex.example.com"},
			{name: "a parent domain", rpID: "example.com"},
			{name: "a typo", rpID: "exmaple.com", wantError: `twoFactor.webAuthn.rpID "exmaple.com" must be the domain of Dex`},
			{name: "a sibling", rpID: "other.example.com", wantError: `twoFactor.webAuthn.rpID "other.example.com"`},
		} {
			Context(tc.name, func() {
				BeforeEach(func() {
					hec.ValuesSet("userAuthn.twoFactor.enabled", true)
					hec.ValuesSet("userAuthn.twoFactor.methods", []string{"WebAuthn"})
					hec.ValuesSet("userAuthn.twoFactor.webAuthn.rpID", tc.rpID)
					hec.HelmRender()
				})

				It("Should accept the Dex domain and its parents only", func() {
					if tc.wantError == "" {
						Expect(hec.RenderError).ToNot(HaveOccurred())
						Expect(gjson.GetBytes(renderedDexConfig(hec), "mfa.authenticators.0.config.rpID").String()).To(Equal(tc.rpID))
						return
					}
					Expect(hec.RenderError).To(HaveOccurred())
					Expect(hec.RenderError.Error()).To(ContainSubstring(tc.wantError))
				})
			})
		}
	})

	Context("With WebAuthn bound to a domain", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.methods", []string{"WebAuthn"})
			hec.ValuesSet("userAuthn.twoFactor.webAuthn.rpID", "example.com")
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"Local", "LDAP"})
			hec.HelmRender()
		})

		It("Should render the relying party ID and ask nothing about the make of the key", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			authenticator := gjson.GetBytes(jsonConfig, "mfa.authenticators.0")
			Expect(authenticator.Get("config.rpID").String()).To(Equal("example.com"))
			Expect(authenticator.Get("config.attestationPreference").String()).To(Equal("none"))
			Expect(authenticator.Get("connectorTypes").Value()).To(Equal([]any{"local", "ldap"}))
			// Dex v2.46 reads these two and uses neither, so they stay out of the config.
			Expect(authenticator.Get("config.userVerification").Exists()).To(BeFalse())
			Expect(authenticator.Get("config.authenticatorAttachment").Exists()).To(BeFalse())
			// The origin is the issuer, whatever the relying party ID is.
			Expect(authenticator.Get("config.rpOrigins").Exists()).To(BeFalse())
		})
	})

	Context("With the method named TOTP", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.methods", []string{"TOTP"})
			hec.ValuesSet("userAuthn.twoFactor.webAuthn.rpID", "example.com")
			hec.HelmRender()
		})

		It("Should ignore the WebAuthn parameters", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.#").Int()).To(BeEquivalentTo(1))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.id").String()).To(Equal("totp"))
			Expect(gjson.GetBytes(jsonConfig, "mfa.defaultMFAChain").Value()).To(Equal([]any{"totp"}))
		})
	})

	// require: Any lets the users of the type choose: the chain stays [totp, webauthn], so a Dex that does not know
	// oneOfConnectorTypes asks for both. A type that requires both is not in the list.
	Context("With a choice of one method", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.methods", []string{"TOTP", "WebAuthn"})
			hec.ValuesSet("userAuthn.twoFactor.require", "Any")
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"Local", "LDAP", "Crowd"})
			hec.ValuesSet("userAuthn.twoFactor.providerTypeOverrides", []map[string]any{
				{"providerType": "LDAP", "methods": []string{"TOTP", "WebAuthn"}, "require": "All"},
			})
			hec.HelmRender()
		})

		It("Should let only the types with Any choose", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.defaultMFAChain").Value()).To(Equal([]any{"totp", "webauthn"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.connectorTypes").Value()).To(Equal([]any{"local", "ldap", "atlassian-crowd"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.oneOfConnectorTypes").Value()).To(Equal([]any{"local", "atlassian-crowd"}))
		})
	})

	Context("With both methods required", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.methods", []string{"TOTP", "WebAuthn"})
			hec.ValuesSet("userAuthn.twoFactor.require", "All")
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"Local", "LDAP"})
			hec.HelmRender()
		})

		It("Should ask for the code and then the key", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.#.id").Value()).To(Equal([]any{"totp", "webauthn"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.connectorTypes").Value()).To(Equal([]any{"local", "ldap"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.1.connectorTypes").Value()).To(Equal([]any{"local", "ldap"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.defaultMFAChain").Value()).To(Equal([]any{"totp", "webauthn"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.oneOfConnectorTypes").Exists()).To(BeFalse())
		})
	})

	Context("With a method for each type of provider", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"Local", "LDAP", "Crowd"})
			hec.ValuesSet("userAuthn.twoFactor.providerTypeOverrides", []map[string]any{
				{"providerType": "LDAP", "methods": []string{"WebAuthn"}},
				{"providerType": "Crowd", "methods": []string{"TOTP", "WebAuthn"}, "require": "All"},
			})
			hec.HelmRender()
		})

		// Dex drops from the chain of a login the authenticators not bound to its connector type: a local user is
		// asked for the code, an LDAP user for the key, a Crowd user for both.
		It("Should bind each authenticator to the types that use it", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.#.id").Value()).To(Equal([]any{"totp", "webauthn"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.connectorTypes").Value()).To(Equal([]any{"local", "atlassian-crowd"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.1.connectorTypes").Value()).To(Equal([]any{"ldap", "atlassian-crowd"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.defaultMFAChain").Value()).To(Equal([]any{"totp", "webauthn"}))
		})
	})

	// Dex applies an authenticator with no connector types to every connector, the OIDC and SAML providers too: a
	// method no type uses must not be rendered with an empty list.
	Context("With a method that no type uses", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.ValuesSet("userAuthn.twoFactor.methods", []string{"TOTP"})
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"Local"})
			hec.ValuesSet("userAuthn.twoFactor.providerTypeOverrides", []map[string]any{{"providerType": "Local", "methods": []string{"WebAuthn"}}})
			hec.HelmRender()
		})

		It("Should leave its authenticator out", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.#.id").Value()).To(Equal([]any{"webauthn"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.connectorTypes").Value()).To(Equal([]any{"local"}))
			Expect(gjson.GetBytes(jsonConfig, "mfa.defaultMFAChain").Value()).To(Equal([]any{"webauthn"}))
		})
	})

	Context("With the providers of 2FA listed but 2FA off", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", false)
			hec.ValuesSet("userAuthn.twoFactor.providerTypes", []string{"LDAP"})
			hec.HelmRender()
		})

		It("Should render no MFA", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(gjson.GetBytes(renderedDexConfig(hec), "mfa").Exists()).To(BeFalse())
		})
	})

	Context("With 2FA and no issuer name", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.twoFactor.enabled", true)
			hec.HelmRender()
		})

		It("Should name the issuer Deckhouse", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonConfig := renderedDexConfig(hec)

			Expect(gjson.GetBytes(jsonConfig, "mfa.authenticators.0.config.issuer").String()).To(Equal("Deckhouse"))
		})
	})

	Context("Without Users but with Providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: gitlabID
  displayName: gitlabName
  type: Gitlab
  gitlab:
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
    groups:
    - Admins
    - Everyone`)
			hec.HelmRender()
		})

		It("Should create dex config without enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).To(BeEmpty())
			Expect(config).NotTo(ContainSubstring("enablePasswordDB: true"))
		})
	})

	Context("With Users but without providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexUsersCRDs", `
- name: userName
- name: adminName
`)
			hec.HelmRender()
		})

		It("Should create dex config with enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).NotTo(BeEmpty())
			Expect(config).To(ContainSubstring("enablePasswordDB: true"))
		})

		It("user-api keeps serving for 5 seconds after SIGTERM", func() {
			userAPI := hec.KubernetesResource("Deployment", "d8-user-authn", "user-api")
			Expect(userAPI.Exists()).To(BeTrue())
			Expect(userAPI.Field(`spec.template.spec.containers.#(name=="user-api").lifecycle.preStop.sleep.seconds`).Int()).To(BeEquivalentTo(5))
		})
	})

	Context("With Users and providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexUsersCRDs", `
- name: userName
- name: adminName
`)
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: gitlabID
  displayName: gitlabName
  type: Gitlab
  gitlab:
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
    groups:
    - Admins
    - Everyone`)
			hec.HelmRender()
		})

		It("Should create dex config with enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).NotTo(BeEmpty())
			Expect(config).To(ContainSubstring("enablePasswordDB: true"))
		})
	})

	Context("With DexProviders and the token exchange flag", func() {
		const providers = `
- id: oidc-unset
  displayName: oidc-unset
  type: OIDC
  oidc: {issuer: "https://idp.example.com", clientID: dex, clientSecret: plainstring}
- id: oidc-on
  displayName: oidc-on
  type: OIDC
  tokenExchange: {enabled: true}
  oidc: {issuer: "https://idp.example.com", clientID: dex, clientSecret: plainstring}
- id: gitlab-off
  displayName: gitlab-off
  type: Gitlab
  tokenExchange: {enabled: false}
  gitlab: {clientID: clientID, clientSecret: secret, baseURL: "https://gitlab.example.com"}
- id: github
  displayName: github
  type: Github
  github: {clientID: clientID, clientSecret: secret}`

		signIn := []any{"authorization_code", "refresh_token", "implicit", "password", "urn:ietf:params:oauth:grant-type:device_code"}
		withExchange := append(append([]any{}, signIn...), "urn:ietf:params:oauth:grant-type:token-exchange")
		grants := func(id string) any {
			return gjson.GetBytes(renderedDexConfig(hec), `connectors.#(id=="`+id+`").grantTypes`).Value()
		}

		It("Before the migration, a provider without the field keeps exchanging tokens", func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", providers)
			hec.ValuesSet("userAuthn.internal.dexProviderTokenExchangeMigrated", false)
			hec.HelmRender()
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(grants("oidc-unset")).To(Equal(withExchange))
			Expect(grants("oidc-on")).To(Equal(withExchange))
			Expect(grants("gitlab-off")).To(Equal(signIn))
			Expect(grants("github")).To(Equal(signIn))
		})

		It("After the migration, only a provider with the flag exchanges tokens", func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", providers)
			hec.ValuesSet("userAuthn.internal.dexProviderTokenExchangeMigrated", true)
			hec.HelmRender()
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(grants("oidc-unset")).To(Equal(signIn))
			Expect(grants("oidc-on")).To(Equal(withExchange))
			Expect(grants("gitlab-off")).To(Equal(signIn))
			Expect(grants("github")).To(Equal(signIn))
		})
	})
})

// renderedDexConfig returns the Dex config of the rendered dex Secret as JSON.
func renderedDexConfig(hec *Config) []byte {
	dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
	configBytes, err := base64.StdEncoding.DecodeString(dexConfig.Field("data.config\\.yaml").String())
	Expect(err).ToNot(HaveOccurred())
	jsonConfig, err := yaml.YAMLToJSON(configBytes)
	Expect(err).ToNot(HaveOccurred())
	return jsonConfig
}
