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

// DexProvider.spec.allowedIdentities is rendered at the connector level, next to type/id/name,
// where dex reads it for every connector type. Absent block -> no keys at all, so a provider
// without limits keeps the exact config it had before the field existed.
var _ = Describe("Module :: user-authn :: helm template :: connector identity limits", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
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

	connectors := func() gjson.Result {
		secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
		data, err := base64.StdEncoding.DecodeString(secret.Field("data.config\\.yaml").String())
		Expect(err).To(BeNil())
		data, err = ConvertYAMLToJSON(data)
		Expect(err).To(BeNil())
		return gjson.GetBytes(data, "connectors")
	}

	Context("With allowedIdentities on OIDC and LDAP providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: contractors
  displayName: Contractors
  type: OIDC
  oidc:
    issuer: https://issuer.example.com
    clientID: clientID
    clientSecret: secret
    allowedGroups:
    - contractors
    - ops
  allowedIdentities:
    emails:
    - Boss@Partner.Example
    emailDomains:
    - contractor.example
    groups:
    - contractors
- id: corp
  displayName: Corp
  type: LDAP
  ldap:
    host: ldap.example.com:636
    bindDN: cn=svc
    bindPW: secret
    userSearch:
      baseDN: dc=example
      username: mail
      idAttr: DN
      emailAttr: mail
  allowedIdentities:
    emailDomains:
    - corp.example
- id: open
  displayName: Open
  type: OIDC
  oidc:
    issuer: https://open.example.com
    clientID: clientID
    clientSecret: secret`)
			hec.HelmRender()
		})

		It("Should render every list at the connector level and leave the connector config as is", func() {
			c := connectors()
			Expect(c.Array()).To(HaveLen(3))

			oidc := c.Get("0")
			Expect(oidc.Get("type").String()).To(Equal("oidc"))
			Expect(oidc.Get("allowedEmails").String()).To(MatchJSON(`["Boss@Partner.Example"]`))
			Expect(oidc.Get("allowedEmailDomains").String()).To(MatchJSON(`["contractor.example"]`))
			Expect(oidc.Get("allowedGroups").String()).To(MatchJSON(`["contractors"]`))
			// the connector's own filter is untouched; dex applies both
			Expect(oidc.Get("config.allowedGroups").String()).To(MatchJSON(`["contractors","ops"]`))
			Expect(oidc.Get("config.allowedEmails").Exists()).To(BeFalse())

			ldap := c.Get("1")
			Expect(ldap.Get("type").String()).To(Equal("ldap"))
			Expect(ldap.Get("allowedEmailDomains").String()).To(MatchJSON(`["corp.example"]`))
			Expect(ldap.Get("allowedEmails").Exists()).To(BeFalse())
			Expect(ldap.Get("allowedGroups").Exists()).To(BeFalse())
		})

		It("Should render nothing for a provider without the block", func() {
			open := connectors().Get("2")
			Expect(open.Get("id").String()).To(Equal("open"))
			Expect(open.Get("allowedEmails").Exists()).To(BeFalse())
			Expect(open.Get("allowedEmailDomains").Exists()).To(BeFalse())
			Expect(open.Get("allowedGroups").Exists()).To(BeFalse())
		})
	})

	Context("With an empty allowedIdentities block", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: empty
  displayName: Empty
  type: OIDC
  oidc:
    issuer: https://issuer.example.com
    clientID: clientID
    clientSecret: secret
  allowedIdentities:
    emails: []
    groups: []`)
			hec.HelmRender()
		})

		It("Should render no limit keys", func() {
			c := connectors().Get("0")
			Expect(c.Get("allowedEmails").Exists()).To(BeFalse())
			Expect(c.Get("allowedEmailDomains").Exists()).To(BeFalse())
			Expect(c.Get("allowedGroups").Exists()).To(BeFalse())
		})
	})
})
