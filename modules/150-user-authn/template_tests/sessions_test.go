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

package template_tests

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// sessionsCookieKey is the key of the session cookie in the tests: a fixed value of the right length.
var sessionsCookieKey = strings.Repeat("k", 32)

// sessionsAuthenticator returns a DexAuthenticator of the internal values with one application.
func sessionsAuthenticator(name, namespace, domain string) string {
	return `
- name: ` + name + `
  encodedName: ` + name + `-encoded
  namespace: ` + namespace + `
  credentials:
    appDexSecret: secret
    cookieSecret: cookie
  spec:
    applications:
    - domain: ` + domain + `
      ingressClassName: nginx
`
}

// sessionsAuthenticatorNames returns the names of the objects of a DexAuthenticator of sessionsAuthenticator.
func sessionsAuthenticatorNames(name, namespace string) string {
	return `
"` + name + `@` + namespace + `":
  name: ` + name + `-dex-authenticator
  secretName: dex-authenticator-` + name + `
  ingressNames:
    "0":
      name: ` + name + `-dex-authenticator
  signOutIngressNames:
    "0":
      name: ` + name + `-dex-authenticator-sign-out
`
}

var _ = Describe("Module :: user-authn :: helm template :: sessions of dex", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexSessionsCookieKey", sessionsCookieKey)
		hec.ValuesSet("userAuthn.idTokenTTL", "15m")
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs",
			sessionsAuthenticator("console", "d8-console", "console.example.com")+
				sessionsAuthenticator("grafana", "d8-monitoring", "grafana.example.com")+
				sessionsAuthenticator("app", "tenant-ns", "app.tenant.com"))
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames",
			sessionsAuthenticatorNames("console", "d8-console")+
				sessionsAuthenticatorNames("grafana", "d8-monitoring")+
				sessionsAuthenticatorNames("app", "tenant-ns"))
		hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: corp
  displayName: Corp
  type: OIDC
  oidc:
    issuer: https://idp.corp.com
    clientID: dex
    clientSecret: secret
    upstreamLogout: true
- id: partner
  displayName: Partner
  type: OIDC
  oidc:
    issuer: https://idp.partner.com
    clientID: dex
    clientSecret: secret
`)
	})

	authenticatorClient := func(name string) object_store.KubeObject {
		client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", name+"-encoded")
		ExpectWithOffset(1, client.Exists()).To(BeTrue())
		return client
	}
	dexEnv := func() gjson.Result {
		return hec.KubernetesResource("Deployment", "d8-user-authn", "dex").
			Field(`spec.template.spec.containers.#(name=="dex").env.#(name=="DEX_SESSIONS_ENABLED")`).Result
	}

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("leaves the sessions of dex off", func() {
			Expect(gjson.GetBytes(renderedDexConfig(hec), "sessions").Exists()).To(BeFalse())
			Expect(dexEnv().Exists()).To(BeFalse())
			Expect(authenticatorArgs(hec, "d8-console", "console-dex-authenticator")).
				NotTo(ContainElement(HavePrefix("--provider-logout-url")))
		})

		It("keeps the cookie key", func() {
			secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex-sessions-cookie-key")
			Expect(secret.Exists()).To(BeTrue())
		})

		It("registers the addresses to return to after sign-out but binds no token to a session", func() {
			client := authenticatorClient("console")
			Expect(client.Field("postLogoutRedirectURIs").String()).To(MatchJSON(`["https://console.example.com/"]`))
			Expect(client.Field("refreshTokenLifetime").Exists()).To(BeFalse())
			Expect(client.Field("ssoSharedWith").Exists()).To(BeFalse())
		})

		It("signs out of an OIDC provider only when asked", func() {
			config := renderedDexConfig(hec)
			Expect(gjson.GetBytes(config, `connectors.#(id=="corp").config.upstreamLogout`).Bool()).To(BeTrue())
			Expect(gjson.GetBytes(config, `connectors.#(id=="partner").config.upstreamLogout`).Exists()).To(BeFalse())
		})
	})

	Context("With sessions", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.sessions.enabled", true)
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("turns the sessions of dex on", func() {
			Expect(dexEnv().Get("value").String()).To(Equal("true"))
			Expect(gjson.GetBytes(renderedDexConfig(hec), "sessions").String()).To(MatchJSON(`{
				"cookieName": "__Host-dex_session",
				"absoluteLifetime": "168h",
				"validIfNotUsedFor": "24h",
				"rememberMeCheckedByDefault": false,
				"ssoSharedWithDefault": "none",
				"cookieEncryptionKey": "` + sessionsCookieKey + `",
				"identityRefresh": {"maxAge": "15m", "gracePeriod": "0s"}
			}`))
		})

		It("binds the tokens of every DexAuthenticator to the session", func() {
			for _, name := range []string{"console", "grafana", "app"} {
				Expect(authenticatorClient(name).Field("refreshTokenLifetime").String()).To(Equal("session"), name)
			}
		})

		It("shares the sign-in between the web interfaces of the platform only", func() {
			Expect(authenticatorClient("console").Field("ssoSharedWith").String()).
				To(MatchJSON(`["dex-authenticator-grafana@d8-monitoring"]`))
			Expect(authenticatorClient("grafana").Field("ssoSharedWith").String()).
				To(MatchJSON(`["dex-authenticator-console@d8-console"]`))
			Expect(authenticatorClient("app").Field("ssoSharedWith").Exists()).To(BeFalse())
		})

		It("signs a DexAuthenticator out through dex", func() {
			Expect(authenticatorArgs(hec, "tenant-ns", "app-dex-authenticator")).
				To(ContainElement("--provider-logout-url=https://dex.example.com/logout"))
		})
	})

	Context("With sessions without SSO and with the freshness of the identity set", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.sessions", `
enabled: true
sso: None
absoluteLifetime: 12h
validIfNotUsedFor: 30m
rememberMeCheckedByDefault: true
identityRefresh:
  maxAge: 5m
  gracePeriod: 1m
`)
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
		})

		It("shares the sign-in with nobody", func() {
			Expect(authenticatorClient("console").Field("ssoSharedWith").Exists()).To(BeFalse())
			Expect(authenticatorClient("console").Field("refreshTokenLifetime").String()).To(Equal("session"))
		})

		It("passes the parameters on", func() {
			Expect(gjson.GetBytes(renderedDexConfig(hec), "sessions").String()).To(MatchJSON(`{
				"cookieName": "__Host-dex_session",
				"absoluteLifetime": "12h",
				"validIfNotUsedFor": "30m",
				"rememberMeCheckedByDefault": true,
				"ssoSharedWithDefault": "none",
				"cookieEncryptionKey": "` + sessionsCookieKey + `",
				"identityRefresh": {"maxAge": "5m", "gracePeriod": "1m"}
			}`))
		})
	})

	// Dex refuses an idle timeout longer than the lifetime: the default of a day yields to a shorter
	// lifetime set alone, and a longer one, or one set explicitly, stays.
	DescribeTable("the idle timeout of the sessions",
		func(settings, absoluteLifetime, validIfNotUsedFor string) {
			hec.ValuesSetFromYaml("userAuthn.sessions", settings)
			hec.HelmRender()
			Expect(hec.RenderError).NotTo(HaveOccurred())
			sessions := gjson.GetBytes(renderedDexConfig(hec), "sessions")
			Expect(sessions.Get("absoluteLifetime").String()).To(Equal(absoluteLifetime))
			Expect(sessions.Get("validIfNotUsedFor").String()).To(Equal(validIfNotUsedFor))
		},
		Entry("a lifetime shorter than a day caps the default", "{enabled: true, absoluteLifetime: 12h}", "12h", "12h"),
		Entry("of hours and minutes too", "{enabled: true, absoluteLifetime: 2h30m}", "2h30m", "2h30m"),
		Entry("a lifetime of a day keeps it", "{enabled: true, absoluteLifetime: 24h}", "24h", "24h"),
		Entry("a longer lifetime keeps it", "{enabled: true, absoluteLifetime: 720h}", "720h", "24h"),
		Entry("an idle timeout set explicitly stays", "{enabled: true, absoluteLifetime: 12h, validIfNotUsedFor: 1h30m}", "12h", "1h30m"),
	)
})
