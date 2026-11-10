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

// ldapSearchProvider is an LDAP provider whose userSearch and groupSearch are given as YAML under the keys of
// the DexProvider spec.
func ldapSearchProvider(userSearch, groupSearch string) string {
	return `
- id: ldapID
  displayName: ldapName
  type: LDAP
  ldap:
    host: ldap.example.com:636
    userSearch:
` + userSearch + `
    groupSearch:
` + groupSearch
}

var _ = Describe("Module :: user-authn :: helm template :: dex LDAP search", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.1")
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

	ldapConnector := func() gjson.Result {
		secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
		Expect(secret.Exists()).To(BeTrue())

		raw, err := base64.StdEncoding.DecodeString(secret.Field("data.config\\.yaml").String())
		Expect(err).ToNot(HaveOccurred())

		data, err := ConvertYAMLToJSON(raw)
		Expect(err).ToNot(HaveOccurred(), "rendered Dex config should be valid YAML")

		connectors := gjson.GetBytes(data, "connectors")
		Expect(connectors.Array()).To(HaveLen(1))
		return connectors.Get("0.config")
	}

	Context("With one attribute in username", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapSearchProvider(`
      baseDN: cn=users,dc=example,dc=com
      username: uid
      idAttr: uid
      emailAttr: mail`, `
      baseDN: cn=groups,dc=example,dc=com
      nameAttr: cn
      userMatchers:
      - userAttr: DN
        groupAttr: member`))
			hec.HelmRender()
		})

		It("Should keep the string and leave out the other fields", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			config := ldapConnector()

			Expect(config.Get("userSearch.username").Value()).To(Equal("uid"))
			Expect(config.Get("userSearch.usernames").Exists()).To(BeFalse())
			Expect(config.Get("groupSearch.userMatchers.0.recursionGroupAttr").Exists()).To(BeFalse())
		})
	})

	Context("With usernames and recursionGroupAttr", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapSearchProvider(`
      baseDN: cn=users,dc=example,dc=com
      usernames: [uid, mail]
      idAttr: uid
      emailAttr: mail`, `
      baseDN: cn=groups,dc=example,dc=com
      nameAttr: cn
      userMatchers:
      - userAttr: DN
        groupAttr: member
        recursionGroupAttr: member`))
			hec.HelmRender()
		})

		It("Should render the list under the username key of Dex and the recursion attribute of the matcher", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			config := ldapConnector()

			Expect(config.Get("userSearch.username").Value()).To(Equal([]any{"uid", "mail"}))
			// Dex refuses a config with a field it does not know, so the key of the CRD must not reach it.
			Expect(config.Get("userSearch.usernames").Exists()).To(BeFalse())
			Expect(config.Get("groupSearch.userMatchers.0.recursionGroupAttr").String()).To(Equal("member"))
			Expect(config.Get("groupSearch.userMatchers.0.groupAttr").String()).To(Equal("member"))
		})
	})
})
