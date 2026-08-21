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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// multilineValue is a DexProvider field value spanning several lines, indented to match the
// `args:` block of the proxy container. Unless it is rendered as a quoted scalar, the newline
// ends the args sequence and the remaining lines become sibling keys of that container.
const multilineValue = "value\n" +
	"        image: registry.example.invalid/injected\n" +
	"        command: [\"/bin/sh\", \"-c\"]\n" +
	"        args: [\"injected\"]\n" +
	"        workingDir: /injected"

const basicAuthProxyImage = "registry.example.com@imageHash-userAuthn-basicAuthProxy"

const crowdProvider = `
- id: crowdID
  displayName: crowdName
  type: Crowd
  crowd:
    enableBasicAuth: true
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
`

const oidcProvider = `
- id: oidcID
  displayName: oidcName
  type: OIDC
  oidc:
    enableBasicAuth: true
    issuer: https://example.com
    clientID: clientID
    clientSecret: secret
`

const ldapProvider = `
- id: ldapID
  displayName: ldapName
  type: LDAP
  ldap:
    enableBasicAuth: true
    host: ldap.example.com:636
    userSearch:
      baseDN: cn=users,dc=example,dc=com
      username: uid
      idAttr: uid
      emailAttr: mail
`

var _ = Describe("Module :: user-authn :: helm template :: basic auth proxy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
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
		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "test")

		hec.ValuesSet("userAuthn.internal.publishAPI.enabled", true)
		hec.ValuesSet("userAuthn.internal.publishAPI.addKubeconfigGeneratorEntry", true)
		hec.ValuesSet("userAuthn.internal.basicAuthProxyCert", "dGVzdA==")
		hec.ValuesSet("userAuthn.internal.basicAuthProxyKey", "dGVzdA==")
	})

	injections := []struct {
		name     string
		provider string
		path     string
		list     bool
		flag     string
	}{
		{"Crowd clientID", crowdProvider, "crowd.clientID", false, "--crowd-application-login"},
		{"Crowd clientSecret", crowdProvider, "crowd.clientSecret", false, "--crowd-application-password"},
		{"Crowd baseURL", crowdProvider, "crowd.baseURL", false, "--crowd-base-url"},
		{"Crowd group", crowdProvider, "crowd.groups", true, "--crowd-allowed-group"},
		{"OIDC issuer", oidcProvider, "oidc.issuer", false, "--oidc-base-url"},
		{"OIDC clientID", oidcProvider, "oidc.clientID", false, "--oidc-client-id"},
		{"OIDC clientSecret", oidcProvider, "oidc.clientSecret", false, "--oidc-client-secret"},
		{"OIDC scope", oidcProvider, "oidc.scopes", true, "--oidc-scope"},
		{"LDAP scope", ldapProvider, "ldap.scopes", true, "--ldap-scope"},
	}

	for _, injection := range injections {
		Context("With a multiline "+injection.name, func() {
			BeforeEach(func() {
				hec.ValuesSetFromYaml("userAuthn.internal.providers", injection.provider)

				path := "userAuthn.internal.providers.0." + injection.path
				if injection.list {
					hec.ValuesSet(path, []string{multilineValue})
				} else {
					hec.ValuesSet(path, multilineValue)
				}

				hec.HelmRender()
			})

			It("Should keep "+injection.flag+" as a single argument of the proxy container", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy")
				Expect(deployment.Exists()).To(BeTrue())

				const container = "spec.template.spec.containers.0"
				Expect(deployment.Field(container + ".image").String()).To(Equal(basicAuthProxyImage))
				Expect(deployment.Field(container + ".command").Exists()).To(BeFalse())
				Expect(deployment.Field(container + ".workingDir").Exists()).To(BeFalse())

				args := deployment.Field(container + ".args").AsStringSlice()
				Expect(args).To(ContainElement("--listen=$(POD_IP):7332"))
				Expect(args).To(ContainElement(injection.flag + "=" + multilineValue))
			})
		})
	}

	Context("With an OIDC provider", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", oidcProvider)
			hec.HelmRender()
		})

		It("Should render boolean arguments as unquoted scalars", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			args := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy").
				Field("spec.template.spec.containers.0.args").AsStringSlice()

			Expect(args).To(ContainElement("--oidc-get-user-info=false"))
			Expect(args).To(ContainElement("--oidc-basic-auth-unsupported=false"))
		})
	})
})
