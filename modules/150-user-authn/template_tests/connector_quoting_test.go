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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// ldapBindPWInjection closes its own scalar with a single quote and continues at the
// indentation of the LDAP connector config, so an unquoted rendering turns the trailing
// lines into extra keys of that connector.
const ldapBindPWInjection = "s3cret'\n" +
	"        insecureNoSSL: true\n" +
	"        injected: 'yes"

// oidcClaimInjection builds a claim mapping value that ends its own scalar and appends an
// extra key at the claimMapping indentation level. Each claim gets its own key name so that
// an unquoted rendering produces distinct keys rather than a duplicate-key parse error.
func oidcClaimInjection(key string) string {
	return "claim'\n          " + key + ": injected"
}

var _ = Describe("Module :: user-authn :: helm template :: connector value quoting", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
	})

	// dexConnectors decodes the rendered Dex configuration file out of its Secret. Parsing it
	// back from YAML also proves the rendered configuration is still well-formed.
	dexConnectors := func() gjson.Result {
		Expect(hec.RenderError).ToNot(HaveOccurred())

		secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
		Expect(secret.Exists()).To(BeTrue())

		raw, err := base64.StdEncoding.DecodeString(secret.Field("data.config\\.yaml").String())
		Expect(err).ToNot(HaveOccurred())

		data, err := ConvertYAMLToJSON(raw)
		Expect(err).ToNot(HaveOccurred(), "rendered Dex config should be valid YAML")

		connectors := gjson.GetBytes(data, "connectors")
		Expect(connectors.Array()).To(HaveLen(1), "no connector should be injected")

		return connectors.Get("0")
	}

	Context("With an LDAP provider whose bindPW contains a quote and a line break", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: ldapID
  displayName: ldapName
  type: LDAP
  ldap:
    host: ldap.example.com:636
    bindDN: uid=serviceaccount,cn=users,dc=example,dc=com
    userSearch:
      baseDN: cn=users,dc=example,dc=com
      username: uid
      idAttr: uid
      emailAttr: mail
`)
			hec.ValuesSet("userAuthn.internal.providers.0.ldap.bindPW", ldapBindPWInjection)
			hec.HelmRender()
		})

		It("Should keep bindPW as a single scalar of the ldap connector", func() {
			connector := dexConnectors()

			Expect(connector.Get("type").String()).To(Equal("ldap"))
			Expect(connector.Get("config.bindPW").String()).To(Equal(ldapBindPWInjection))
			Expect(connector.Get("config.injected").Exists()).To(BeFalse())
			Expect(connector.Get("config.insecureNoSSL").Exists()).To(BeFalse())
		})
	})

	Context("With an OIDC provider whose claim mapping contains quotes and line breaks", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: oidcID
  displayName: oidcName
  type: OIDC
  oidc:
    issuer: https://issuer.example.com
    clientID: clientID
    clientSecret: secret
`)
			hec.ValuesSet("userAuthn.internal.providers.0.oidc.claimMapping.email", oidcClaimInjection("injectedEmail"))
			hec.ValuesSet("userAuthn.internal.providers.0.oidc.claimMapping.groups", oidcClaimInjection("injectedGroups"))
			hec.ValuesSet("userAuthn.internal.providers.0.oidc.claimMapping.preferred_username", oidcClaimInjection("injectedUsername"))
			hec.HelmRender()
		})

		It("Should keep every claim mapping as a single scalar of the oidc connector", func() {
			connector := dexConnectors()

			Expect(connector.Get("type").String()).To(Equal("oidc"))

			claimMapping := connector.Get("config.claimMapping")
			Expect(claimMapping.Get("email").String()).To(Equal(oidcClaimInjection("injectedEmail")))
			Expect(claimMapping.Get("groups").String()).To(Equal(oidcClaimInjection("injectedGroups")))
			Expect(claimMapping.Get("preferred_username").String()).To(Equal(oidcClaimInjection("injectedUsername")))

			for _, key := range []string{"injectedEmail", "injectedGroups", "injectedUsername"} {
				Expect(claimMapping.Get(key).Exists()).To(BeFalse())
				Expect(connector.Get("config." + key).Exists()).To(BeFalse())
			}
		})
	})
})
