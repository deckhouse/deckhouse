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
	"fmt"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// authLogLine renders an auth log line the way the JSON logger of patch 007 writes it.
// The User-Agent carries the text of another event in every line: encoding/json escapes
// its quotes, so it must never decide which event a line produces.
func authLogLine(user, status, msg string) string {
	return `{"time":"2026/10/22 13:25:45","level":"auth","client":"10.1.2.3","host":"app.example.com",` +
		`"protocol":"HTTP/1.1","request_id":"4c2f7a","method":"GET",` +
		`"user_agent":"Mozilla/5.0 \"status\":\"AuthSuccess\",\"msg\":\"Authenticated via OAuth2: x",` +
		`"user":"` + user + `","status":"` + status + `","msg":"` + msg + `"}`
}

// setDexAuthenticators sets the internal values of one DexAuthenticator per namespace,
// with the names the hooks would have computed for them.
func setDexAuthenticators(hec *Config, namespaces ...string) {
	crds := make([]map[string]any, 0, len(namespaces))
	names := map[string]any{}
	for i, ns := range namespaces {
		name := fmt.Sprintf("app%d", i)
		crds = append(crds, map[string]any{
			"name":        name,
			"encodedName": name,
			"namespace":   ns,
			"credentials": map[string]any{"appDexSecret": "dexSecret", "cookieSecret": "cookieSecret"},
			"spec": map[string]any{
				"applications": []map[string]any{{
					"domain":            name + ".example.com",
					"ingressClassName":  "test",
					"ingressSecretName": "test",
				}},
			},
		})
		names[name+"@"+ns] = map[string]any{
			"name":            name + "-dex-authenticator",
			"truncated":       false,
			"hash":            "",
			"secretName":      "dex-authenticator-" + name,
			"secretTruncated": false,
			"secretHash":      "",
			"ingressNames": map[string]any{
				"0": map[string]any{"name": name + "-dex-authenticator", "truncated": false, "hash": ""},
			},
		}
	}
	hec.ValuesSet("userAuthn.internal.dexAuthenticatorCRDs", crds)
	hec.ValuesSet("userAuthn.internal.dexAuthenticatorNames", names)
}

var _ = Describe("Module :: user-authn :: helm template :: security events", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
	})

	Context("Without the security-events-manager API", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})

		It("Should not render the shipper", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesGlobalResource("ClusterSecurityEventShipper", "dex-authenticator").Exists()).To(BeFalse())
			Expect(hec.KubernetesGlobalResource("SecurityEventDefinition", "dex-authenticator-login-succeeded").Exists()).To(BeFalse())
		})
	})

	Context("With the security-events-manager API and no DexAuthenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("global.discovery.apiVersions", `["security.deckhouse.io/v1alpha1/SecurityEventDefinition"]`)
			hec.HelmRender()
		})

		// Without names the namespace selector would match every namespace.
		It("Should render the definitions but not the shipper", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesGlobalResource("SecurityEventDefinition", "dex-authenticator-login-succeeded").Exists()).To(BeTrue())
			Expect(hec.KubernetesGlobalResource("ClusterSecurityEventShipper", "dex-authenticator").Exists()).To(BeFalse())
		})
	})

	Context("With DexAuthenticators in more namespaces than one shipper item takes", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("global.discovery.apiVersions", `["security.deckhouse.io/v1alpha1/SecurityEventDefinition"]`)
			namespaces := make([]string, 0, 300)
			for i := 0; i < 300; i++ {
				namespaces = append(namespaces, fmt.Sprintf("ns-%03d", i))
			}
			setDexAuthenticators(hec, namespaces...)
			hec.HelmRender()
		})

		It("Should split the namespaces into items of one shipper with the same rules", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			items := hec.KubernetesGlobalResource("ClusterSecurityEventShipper", "dex-authenticator").Field("spec").Array()
			Expect(items).To(HaveLen(2))
			Expect(items[0].Get("input.kubernetesPods.namespaceSelector.matchNames").Array()).To(HaveLen(256))
			Expect(items[1].Get("input.kubernetesPods.namespaceSelector.matchNames").Array()).To(HaveLen(44))
			Expect(items[1].Get("source").String()).To(Equal(items[0].Get("source").String()))
			Expect(items[1].Get("produces").Raw).To(Equal(items[0].Get("produces").Raw))
			Expect(items[1].Get("producesDefaults").Raw).To(Equal(items[0].Get("producesDefaults").Raw))
		})
	})

	Context("With the security-events-manager API", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("global.discovery.apiVersions", `["security.deckhouse.io/v1alpha1/SecurityEventDefinition"]`)
			setDexAuthenticators(hec, "app-b", "app-a", "app-b")
			hec.HelmRender()
		})

		It("Should read the pods of the DexAuthenticator namespaces only", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			items := hec.KubernetesGlobalResource("ClusterSecurityEventShipper", "dex-authenticator").Field("spec").Array()
			Expect(items).To(HaveLen(1))
			Expect(items[0].Get("input.kubernetesPods.namespaceSelector.matchNames").Raw).To(MatchJSON(`["app-a","app-b"]`))
		})

		It("Should define every event the shipper produces", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			shipper := hec.KubernetesGlobalResource("ClusterSecurityEventShipper", "dex-authenticator")
			Expect(shipper.Exists()).To(BeTrue())
			Expect(shipper.Field("spec.0.input.kubernetesPods.labelSelector.matchLabels.app").String()).To(Equal("dex-authenticator"))

			produces := shipper.Field("spec.0.produces").Array()
			Expect(produces).ToNot(BeEmpty())
			for _, p := range produces {
				code := p.Get("eventCode").String()
				name := strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(code, "D8_")), "_", "-")
				definition := hec.KubernetesGlobalResource("SecurityEventDefinition", name)
				Expect(definition.Exists()).To(BeTrue(), code)
				Expect(definition.Field("spec.code").String()).To(Equal(code))
				Expect(definition.Field("spec.source").String()).To(Equal("dex-authenticator"), code)
				// The Russian texts come from i18n/security-events.yaml by the name of the definition.
				Expect(definition.Field("spec.descriptionRu").String()).To(MatchRegexp(`\p{Cyrillic}`), code)
				Expect(definition.Field("spec.metadata.docs.desc.ru").String()).To(MatchRegexp(`\p{Cyrillic}`), code)

				// A produces[].enrich list replaces producesDefaults.enrich, so every
				// event has to carry the shared rules itself.
				var targets []string
				for _, e := range p.Get("enrich").Array() {
					targets = append(targets, e.Get("target").String())
				}
				Expect(targets).To(ContainElements("event.outcome", "source.component", "actor.type", "object.type"), code)
			}
		})

		It("Should map each auth log line to exactly one event", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			lines := map[string]string{
				authLogLine("jane@example.com", "AuthSuccess", "Authenticated via OAuth2: Session{email:jane@example.com user:Cgcx groups:[admins]}"): "D8_DEX_AUTHENTICATOR_LOGIN_SUCCEEDED",
				authLogLine("jane@example.com", "AuthSuccess", "User signed out"):                                                                     "D8_DEX_AUTHENTICATOR_LOGOUT",
				authLogLine("jane@example.com", "AuthFailure", "Invalid authentication via OAuth2: unauthorized"):                                     "D8_DEX_AUTHENTICATOR_LOGIN_DENIED",
				authLogLine("jane@example.com", "AuthFailure", "Session validation failed: Session{}"):                                                "D8_DEX_AUTHENTICATOR_LOGIN_DENIED",
				authLogLine("jane@example.com", "AuthFailure", "Invalid authorization via session (unauthorized): removing session Session{}"):        "D8_DEX_AUTHENTICATOR_LOGIN_DENIED",
				authLogLine("-", "AuthFailure", "Invalid authentication via OAuth2: unable to obtain CSRF cookie"):                                    "D8_DEX_AUTHENTICATOR_LOGIN_FAILED",
				authLogLine("-", "AuthFailure", "Invalid authentication via OAuth2: identity provider returned an error: access_denied"):              "D8_DEX_AUTHENTICATOR_LOGIN_FAILED",
				authLogLine("-", "AuthError", "Invalid authentication via OAuth2: unable to redeem the code"):                                         "D8_DEX_AUTHENTICATOR_LOGIN_FAILED",
				authLogLine("jane@example.com", "AuthFailure", "Invalid authentication via OAuth2: CSRF token mismatch, potential attack"):            "D8_DEX_AUTHENTICATOR_CSRF_MISMATCH",
				authLogLine("jane@example.com", "AuthError", `Unable to refresh session: error refreshing tokens: oauth2: \"invalid_request\"`):       "D8_DEX_AUTHENTICATOR_SESSION_REFRESH_FAILED",
				`{"time":"2026/10/22 13:25:45","level":"error","msg":"Error while parsing OAuth2 callback: access_denied"}`:                           "",
			}

			produces := hec.KubernetesGlobalResource("ClusterSecurityEventShipper", "dex-authenticator").Field("spec.0.produces").Array()
			for line, expected := range lines {
				var matched []string
				for _, p := range produces {
					for _, v := range p.Get("extract.values").Array() {
						if regexp.MustCompile(v.String()).MatchString(line) {
							matched = append(matched, p.Get("eventCode").String())
							break
						}
					}
				}
				if expected == "" {
					Expect(matched).To(BeEmpty(), line)
				} else {
					Expect(matched).To(Equal([]string{expected}), line)
				}
			}
		})
	})
})
