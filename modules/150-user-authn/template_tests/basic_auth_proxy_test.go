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

	// A pod cannot always reach the public address of Dex (no hairpin NAT), and the proxy crashed
	// on start when it could not: requests go to the Dex Service, and its certificate is verified.
	Context("With an LDAP provider", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			hec.HelmRender()
		})

		// The proxy checks the password with the password grant and never refreshes: a refresh token
		// would only rewrite the offline session of the user at every check.
		It("Should not ask Dex for a refresh token", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			args := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy").
				Field(`spec.template.spec.containers.#(name=="proxy").args`).AsStringSlice()
			Expect(args).To(ContainElements("--ldap-scope=openid", "--ldap-scope=groups", "--ldap-scope=federated:id"))
			Expect(args).ToNot(ContainElement("--ldap-scope=offline_access"))
		})

		It("Should send the requests to the Dex Service and verify its certificate with the Dex CA", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			args := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy").
				Field("spec.template.spec.containers.0.args").AsStringSlice()
			Expect(args).To(ContainElement("--ldap-base-url=https://dex.example.com/"))
			Expect(args).To(ContainElement("--ldap-dex-url=https://dex.d8-user-authn/"))
			Expect(args).To(ContainElement("--ldap-ca-file=/etc/certs/dex-ca.crt"))

			secret := hec.KubernetesResource("Secret", "d8-user-authn", "basic-auth-cert")
			Expect(secret.Field(`data.dex-ca\.crt`).String()).To(Equal("cGxhaW5zdHJpbmc="))
		})
	})

	Context("With an OIDC provider", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", oidcProvider)
			hec.HelmRender()
		})

		It("Should keep serving for 5 seconds after SIGTERM", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			proxy := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy")
			Expect(proxy.Field(`spec.template.spec.containers.#(name=="proxy").lifecycle.preStop.sleep.seconds`).Int()).To(BeEquivalentTo(5))
		})

		It("Should render boolean arguments as unquoted scalars", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			args := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy").
				Field("spec.template.spec.containers.0.args").AsStringSlice()

			Expect(args).To(ContainElement("--oidc-get-user-info=false"))
			Expect(args).To(ContainElement("--oidc-basic-auth-unsupported=false"))
		})

		It("Should not render the SecurityPolicyException without its API", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("SecurityPolicyException", "d8-system", "user-authn-proxy-cert-generate-job").Exists()).To(BeFalse())
		})
	})

	Context("With an OIDC provider and the SecurityPolicyException API", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			hec.ValuesSetFromYaml("userAuthn.internal.providers", oidcProvider)
			hec.HelmRender()
		})

		It("Should allow the proxy-cert-generate-job Job only root and the read-only front-proxy CA files", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			// The name matches the label that hooks/generate_basic_auth_proxy_cert.go sets on the Job pod.
			spe := hec.KubernetesResource("SecurityPolicyException", "d8-system", "user-authn-proxy-cert-generate-job")
			Expect(spe.Exists()).To(BeTrue())
			Expect(spe.Field("spec.securityContext.runAsUser.allowedValues").String()).To(MatchJSON(`[0]`))
			// MustRunAsNonRoot is excepted only when both runAsUser and runAsNonRoot are allowed.
			Expect(spe.Field("spec.securityContext.runAsNonRoot").Exists()).To(BeTrue())
			Expect(spe.Field("spec.securityContext.runAsNonRoot.allowedValue").Bool()).To(BeFalse())
			Expect(spe.Field("spec.volumes.types.allowedValues").String()).To(MatchJSON(`["hostPath"]`))
			Expect(spe.Field("spec.volumes.hostPath.allowedValues.#.path").String()).To(MatchJSON(`["/etc/kubernetes/pki/front-proxy-ca.crt","/etc/kubernetes/pki/front-proxy-ca.key"]`))
			Expect(spe.Field("spec.volumes.hostPath.allowedValues.#.readOnly").String()).To(MatchJSON(`[true,true]`))
			Expect(spe.Field("spec.network").Exists()).To(BeFalse())
		})
	})

	Context("With an LDAP provider", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			hec.HelmRender()
		})

		It("Should let the proxy follow the revocations in its namespace", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			role := hec.KubernetesResource("Role", "d8-user-authn", "basic-auth-proxy")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchJSON(`[{
				"apiGroups": ["dex.coreos.com"],
				"resources": ["revocations"],
				"verbs": ["get", "list", "watch"]
			}]`))

			roleBinding := hec.KubernetesResource("RoleBinding", "d8-user-authn", "basic-auth-proxy")
			Expect(roleBinding.Exists()).To(BeTrue())
			Expect(roleBinding.Field("roleRef.kind").String()).To(Equal("Role"))
			Expect(roleBinding.Field("roleRef.name").String()).To(Equal("basic-auth-proxy"))
			Expect(roleBinding.Field("subjects").String()).To(MatchJSON(`[{
				"kind": "ServiceAccount",
				"name": "basic-auth-proxy",
				"namespace": "d8-user-authn"
			}]`))

			// The informer authenticates with the token of the service account.
			deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy")
			Expect(deployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("basic-auth-proxy"))
			Expect(deployment.Field("spec.template.spec.automountServiceAccountToken").Bool()).To(BeTrue())
		})
	})

	// M4: the proxy sends the password grant to Dex inside the cluster with the address of the
	// end client and the secret Dex trusts it on. The secret is in the Secret of the proxy and in
	// its environment, never in its arguments.
	Context("With an LDAP provider and the trusted proxy secret of Dex", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			hec.ValuesSet("userAuthn.internal.dexTrustedProxySecret", "trusted-proxy-secret")
			hec.HelmRender()
		})

		It("Should give the proxy the in-cluster address of Dex and the secret", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			secret := hec.KubernetesResource("Secret", "d8-user-authn", "basic-auth-cert")
			Expect(secret.Exists()).To(BeTrue())
			// base64 of trusted-proxy-secret
			Expect(secret.Field("data.dex-trusted-proxy-secret").String()).To(Equal("dHJ1c3RlZC1wcm94eS1zZWNyZXQ="))

			proxy := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy")
			const container = `spec.template.spec.containers.#(name=="proxy")`
			args := proxy.Field(container + ".args").AsStringSlice()
			Expect(args).To(ContainElement("--ldap-dex-url=https://dex.d8-user-authn/"))
			Expect(proxy.Field(container + ".args").String()).ToNot(ContainSubstring("trusted-proxy-secret"))
			Expect(proxy.Field(container + `.env.#(name=="DEX_TRUSTED_PROXY_SECRET")`).String()).To(MatchJSON(`{
				"name": "DEX_TRUSTED_PROXY_SECRET",
				"valueFrom": {"secretKeyRef": {"name": "basic-auth-cert", "key": "dex-trusted-proxy-secret", "optional": true}}
			}`))
		})
	})

	Context("With an LDAP provider without the trusted proxy secret of Dex", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			hec.HelmRender()
		})

		It("Should leave the secret out", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			secret := hec.KubernetesResource("Secret", "d8-user-authn", "basic-auth-cert")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("data.dex-trusted-proxy-secret").Exists()).To(BeFalse())
			// The variable is optional: the proxy starts and sends no secret.
			proxy := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy")
			Expect(proxy.Field(`spec.template.spec.containers.#(name=="proxy").env.#(name=="DEX_TRUSTED_PROXY_SECRET").valueFrom.secretKeyRef.optional`).Bool()).To(BeTrue())
		})
	})

	// Crowd and OIDC check passwords past Dex: a revocation does not apply to them, and the proxy
	// does not read revocations.
	for _, c := range []struct {
		name     string
		provider string
	}{
		{"Crowd", crowdProvider},
		{"OIDC", oidcProvider},
	} {
		Context("With a "+c.name+" provider", func() {
			BeforeEach(func() {
				hec.ValuesSetFromYaml("userAuthn.internal.providers", c.provider)
				hec.HelmRender()
			})

			It("Should not let the proxy read revocations", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())
				proxy := hec.KubernetesResource("Deployment", "d8-user-authn", "basic-auth-proxy")
				Expect(proxy.Exists()).To(BeTrue())
				// Nothing of Dex for a provider that does not go through Dex.
				Expect(proxy.Field(`spec.template.spec.containers.#(name=="proxy").env.#(name=="DEX_TRUSTED_PROXY_SECRET")`).Exists()).To(BeFalse())
				Expect(proxy.Field(`spec.template.spec.containers.#(name=="proxy").args`).String()).ToNot(ContainSubstring("--ldap-dex-url"))
				Expect(hec.KubernetesResource("Role", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeFalse())
				Expect(hec.KubernetesResource("RoleBinding", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeFalse())
			})
		})
	}

	Context("With an LDAP provider without basic auth", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			hec.ValuesSet("userAuthn.internal.providers.0.ldap.enableBasicAuth", false)
			hec.HelmRender()
		})

		It("Should not let the proxy read revocations", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("Role", "d8-user-authn", "basic-auth-proxy").Exists()).To(BeFalse())
		})
	})

	// The hook creates the Job before Helm runs, so the exception must already exist
	// when basic authentication is turned on.
	speRenderedCases := []struct {
		name      string
		providers string
		publish   bool
	}{
		{"without providers", `[]`, true},
		{"with basic auth disabled for the provider", `
- id: oidcID
  displayName: oidcName
  type: OIDC
  oidc:
    enableBasicAuth: false
    issuer: https://example.com
    clientID: clientID
    clientSecret: secret
`, true},
		{"with publishAPI disabled", oidcProvider, false},
	}

	for _, c := range speRenderedCases {
		Context("Basic auth is not used "+c.name, func() {
			BeforeEach(func() {
				hec.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
				hec.ValuesSet("userAuthn.internal.publishAPI.enabled", c.publish)
				hec.ValuesSetFromYaml("userAuthn.internal.providers", c.providers)
				hec.HelmRender()
			})

			It("Should still render the SecurityPolicyException", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())
				Expect(hec.KubernetesResource("SecurityPolicyException", "d8-system", "user-authn-proxy-cert-generate-job").Exists()).To(BeTrue())
			})
		})
	}
})
