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
	"fmt"

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

	Context("With groupPrefix", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: corp-ldap
  displayName: Corp
  type: LDAP
  groupPrefix: "corp-ldap:"
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
    groups:
    - "corp-ldap:staff"
- id: gitlab
  displayName: GitLab
  type: Gitlab
  groupPrefix: "gl:"
  gitlab:
    clientID: clientID
    clientSecret: secret
    baseURL: https://gitlab.example.com
- id: idp
  displayName: Idp
  type: OIDC
  groupPrefix: "it's:"
  oidc:
    issuer: https://issuer.example.com
    clientID: clientID
    clientSecret: secret
- id: open
  displayName: Open
  type: OIDC
  oidc:
    issuer: https://open.example.com
    clientID: clientID
    clientSecret: secret`)
			hec.HelmRender()
		})

		It("Should render the prefix at the connector level for every type, next to the lists that compare with it", func() {
			c := connectors()
			Expect(c.Array()).To(HaveLen(4))

			ldap := c.Get("0")
			Expect(ldap.Get("groupPrefix").String()).To(Equal("corp-ldap:"))
			Expect(ldap.Get("allowedGroups").String()).To(MatchJSON(`["corp-ldap:staff"]`))
			// the prefix is a limit of the connector, not an option of its config
			Expect(ldap.Get("config.groupPrefix").Exists()).To(BeFalse())

			Expect(c.Get("1.groupPrefix").String()).To(Equal("gl:"))
			Expect(c.Get("1.config.groupPrefix").Exists()).To(BeFalse())
		})

		It("Should quote the prefix", func() {
			Expect(connectors().Get("2.groupPrefix").String()).To(Equal("it's:"))
		})

		It("Should put no prefix in front of the groups of a provider without one", func() {
			open := connectors().Get("3")
			Expect(open.Get("id").String()).To(Equal("open"))
			Expect(open.Get("groupPrefix").Exists()).To(BeFalse())
		})
	})

	// The webhooks compare each DexProvider with a snapshot of the others, and one apply of two may pass them: the
	// render refuses two prefixes of which one starts with the other.
	Context("With group prefixes that meet", func() {
		for _, tc := range []struct {
			name      string
			prefixes  []string
			wantError string
		}{
			{name: "equal", prefixes: []string{"corp:", "corp:"}, wantError: `groupPrefix "corp:" of the DexProvider "p1" can name the same groups as groupPrefix "corp:" of the DexProvider "p0"`},
			{name: "the second starts with the first", prefixes: []string{"corp:", "corp:ldap:"}, wantError: `groupPrefix "corp:ldap:" of the DexProvider "p1"`},
			{name: "the first starts with the second", prefixes: []string{"corp:ldap:", "corp:"}, wantError: `groupPrefix "corp:" of the DexProvider "p1"`},
			{name: "apart", prefixes: []string{"corp:", "corp2:", "gl:"}},
			{name: "one without a prefix", prefixes: []string{"corp:", ""}},
		} {
			Context(tc.name, func() {
				BeforeEach(func() {
					providers := ""
					for i, prefix := range tc.prefixes {
						providers += fmt.Sprintf("\n- id: p%d\n  displayName: P%d\n  type: OIDC\n  oidc: {issuer: https://idp%d.example.com, clientID: id, clientSecret: secret}", i, i, i)
						if prefix != "" {
							providers += fmt.Sprintf("\n  groupPrefix: %q", prefix)
						}
					}
					hec.ValuesSetFromYaml("userAuthn.internal.providers", providers)
					hec.HelmRender()
				})

				It("Should refuse the render only when one prefix starts with another", func() {
					if tc.wantError == "" {
						Expect(hec.RenderError).ToNot(HaveOccurred())
						return
					}
					Expect(hec.RenderError).To(HaveOccurred())
					Expect(hec.RenderError.Error()).To(ContainSubstring(tc.wantError))
				})
			})
		}
	})
})
