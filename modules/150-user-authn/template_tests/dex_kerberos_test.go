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

// kerberosProviders is an LDAP provider with the given kerberos block next to an OIDC provider, so a
// test can tell that one provider's Kerberos settings leave the other connectors alone.
func kerberosProviders(kerberos string) string {
	return `
- id: ldapID
  displayName: ldapName
  type: LDAP
  ldap:
    host: ldap.example.com:636
    userSearch:
      baseDN: cn=users,dc=example,dc=com
      username: uid
      idAttr: uid
      emailAttr: mail
    kerberos:
` + kerberos + `
- id: oidcID
  displayName: oidcName
  type: OIDC
  oidc:
    issuer: https://issuer.example.com
    clientID: clientID
    clientSecret: secret
`
}

var _ = Describe("Module :: user-authn :: helm template :: dex kerberos", func() {
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

	// dexConnectors decodes the connectors of the rendered Dex configuration out of its Secret.
	dexConnectors := func() gjson.Result {
		secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
		Expect(secret.Exists()).To(BeTrue())

		raw, err := base64.StdEncoding.DecodeString(secret.Field("data.config\\.yaml").String())
		Expect(err).ToNot(HaveOccurred())

		data, err := ConvertYAMLToJSON(raw)
		Expect(err).ToNot(HaveOccurred(), "rendered Dex config should be valid YAML")

		connectors := gjson.GetBytes(data, "connectors")
		Expect(connectors.Array()).To(HaveLen(2))
		Expect(connectors.Get("0.id").String()).To(Equal("ldapID"))
		Expect(connectors.Get("1.id").String()).To(Equal("oidcID"))

		return connectors
	}

	const (
		keytabVolume    = `spec.template.spec.volumes.#(name=="kerberos-keytab-ldapID")`
		keytabMount     = `spec.template.spec.containers.#(name=="dex").volumeMounts.#(name=="kerberos-keytab-ldapID")`
		keytabMountPath = "/etc/dex/kerberos/ldapID"
	)

	Context("With Kerberos enabled and a keytab Secret", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", kerberosProviders(`
      enabled: true
      keytabSecretName: dex-keytab
`))
			hec.HelmRender()
		})

		It("Should enable Kerberos in the connector and mount the keytab", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			kerberos := dexConnectors().Get("0.config.kerberos")
			Expect(kerberos.Get("enabled").Bool()).To(BeTrue())
			Expect(kerberos.Get("keytabPath").String()).To(Equal(keytabMountPath + "/krb5.keytab"))

			deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "dex")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field(keytabVolume + ".secret.secretName").String()).To(Equal("dex-keytab"))
			Expect(deployment.Field(keytabMount + ".mountPath").String()).To(Equal(keytabMountPath))
		})
	})

	Context("With Kerberos enabled and no keytab Secret", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", kerberosProviders(`
      enabled: true
      expectedRealm: EXAMPLE.COM
`))
			hec.HelmRender()
		})

		It("Should render Dex with Kerberos off for that provider only", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			connectors := dexConnectors()
			Expect(connectors.Get("0.config.kerberos.enabled").Bool()).To(BeFalse())
			Expect(connectors.Get("0.config.kerberos.expectedRealm").String()).To(Equal("EXAMPLE.COM"))
			Expect(connectors.Get("1.type").String()).To(Equal("oidc"))

			deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "dex")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field(keytabVolume).Exists()).To(BeFalse())
			Expect(deployment.Field(keytabMount).Exists()).To(BeFalse())
		})
	})

	Context("With Kerberos disabled and its settings kept", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", kerberosProviders(`
      enabled: false
      keytabSecretName: dex-keytab
      fallbackToPassword: true
`))
			hec.HelmRender()
		})

		It("Should keep Kerberos off and mount nothing", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			kerberos := dexConnectors().Get("0.config.kerberos")
			Expect(kerberos.Get("enabled").Bool()).To(BeFalse())
			Expect(kerberos.Get("fallbackToPassword").Bool()).To(BeTrue())

			deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "dex")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field(keytabVolume).Exists()).To(BeFalse())
			Expect(deployment.Field(keytabMount).Exists()).To(BeFalse())
		})
	})
})
