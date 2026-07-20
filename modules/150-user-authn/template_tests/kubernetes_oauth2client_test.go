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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: kubernetes oauth2client", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
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
	})

	Context("Without dex authenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `[]`)
			hec.HelmRender()
		})
		It("Should not deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeFalse())
		})
	})

	Context("With dex authenticator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  allowAccessToKubernetes: true
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"test@d8-test":
  name: "test-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-test"
  secretTruncated: false
  secretHash: ""
  ingressNames: {}
`)
			hec.HelmRender()
		})
		It("Should deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeTrue())
		})
	})

	Context("With dex authenticator without access to Kubernetes API", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: test
  encodedName: justForTest
  namespace: d8-test
  credentials:
    appDexSecret: dexSecret
    cookieSecret: cookieSecret
  spec:
    applicationDomain: authenticator.example.com
    applicationIngressCertificateSecretName: test
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"test@d8-test":
  name: "test-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-test"
  secretTruncated: false
  secretHash: ""
  ingressNames: {}
`)
			hec.HelmRender()
		})
		It("Should not deploy kubernetes OAuth2Client", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk").Exists()).To(BeFalse())
		})
	})

	Context("Full scenario: dex-authenticator + dex-client + publishAPI + kubeconfigGenerator", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
- name: with-access
  encodedName: encWithAccess
  allowAccessToKubernetes: true
  namespace: d8-with-access
  credentials:
    appDexSecret: s1
    cookieSecret: c1
  spec:
    applicationDomain: with-access.example.com
    applicationIngressCertificateSecretName: cert-1
- name: no-access
  encodedName: encNoAccess
  namespace: d8-no-access
  credentials:
    appDexSecret: s2
    cookieSecret: c2
  spec:
    applicationDomain: no-access.example.com
    applicationIngressCertificateSecretName: cert-2
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"with-access@d8-with-access":
  name: "with-access-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-with-access"
  secretTruncated: false
  secretHash: ""
  ingressNames: {}
"no-access@d8-no-access":
  name: "no-access-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-no-access"
  secretTruncated: false
  secretHash: ""
  ingressNames: {}
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexClientCRDs", `
- id: my-app@d8-test
  encodedID: nvqxe33pftwgs5dpojstkylhojpwggrcgji
  legacyID: my-app:d8-test
  legacyEncodedID: nvqxe33pfvtwk3tfmrqxe33pftwgs5dpojstkylhojpwggrcgji
  name: my-app
  namespace: d8-test
  clientSecret: my-app-secret
  spec:
    redirectURIs:
    - https://my-app.example.com/callback
    trustedPeers: []
    allowedGroups: []
  labels: {}
  annotations: {}
  allowAccessToKubernetes: true
- id: my-app-no-k8s@d8-test
  encodedID: nvqxe33pftxhi33mmuxgs5dpojstkylhojpwggrcgji
  legacyID: my-app-no-k8s:d8-test
  legacyEncodedID: nvqxe33pftxhi33mmuxgs5dpojstkylhojpwggrcgji-legacy
  name: my-app-no-k8s
  namespace: d8-test
  clientSecret: my-app-no-k8s-secret
  spec:
    redirectURIs:
    - https://no-k8s.example.com/callback
    trustedPeers: []
    allowedGroups: []
  labels: {}
  annotations: {}
  allowAccessToKubernetes: false
`)
			hec.ValuesSet("userAuthn.internal.publishAPI.enabled", true)
			hec.ValuesSet("userAuthn.internal.publishAPI.addKubeconfigGeneratorEntry", true)
			hec.ValuesSet("userAuthn.internal.publishAPI.publishedAPIKubeconfigGeneratorMasterCA", "publish-api-ca")

			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.id", "plain")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.masterURI", "https://plain.master")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.description", "plain desc")

			hec.ValuesSet("userAuthn.kubeconfigGenerator.1.id", "prod:eu")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.1.masterURI", "https://prod-eu.master")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.1.description", "prod eu desc")

			hec.ValuesSet("userAuthn.internal.kubeconfigEncodedNames", []string{
				encoding.ToFnvLikeDex("kubeconfig-generator-0"),
				encoding.ToFnvLikeDex("kubeconfig-generator-1"),
			})
			hec.ValuesSet("userAuthn.internal.kubeconfigClientEncodedNames", []string{
				encoding.ToFnvLikeDex("kubeconfig-plain"),
				encoding.ToFnvLikeDex("kubeconfig-prod-eu"),
			})
			hec.ValuesSet("userAuthn.internal.kubeconfigPublishAPIEncodedName",
				encoding.ToFnvLikeDex("kubeconfig-publish-api"))

			hec.HelmRender()
		})

		It("Should render OAuth2Client with all expected trustedPeers, redirectURIs and secret", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			oauth2Client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk")
			Expect(oauth2Client.Exists()).To(BeTrue())

			Expect(oauth2Client.Field("id").String()).To(Equal("kubernetes"))
			Expect(oauth2Client.Field("name").String()).To(Equal("kubernetes"))
			Expect(oauth2Client.Field("secret").String()).To(Equal("plainstring"))

			peers := []string{}
			for _, p := range oauth2Client.Field("trustedPeers").Array() {
				peers = append(peers, p.String())
			}
			Expect(peers).To(ConsistOf(
				"with-access-d8-with-access-dex-authenticator",
				"my-app@d8-test",
				"kubeconfig-generator",
				"kubeconfig-publish-api",
				"kubeconfig-generator-0",
				"kubeconfig-plain",
				"kubeconfig-generator-1",
				"kubeconfig-prod-eu",
			))

			uris := []string{}
			for _, u := range oauth2Client.Field("redirectURIs").Array() {
				uris = append(uris, u.String())
			}
			Expect(uris).To(ConsistOf(
				"https://kubeconfig.example.com/callback/0",
				"https://kubeconfig.example.com/callback/1",
				"https://kubeconfig.example.com/callback/",
				"https://with-access.example.com/dex-authenticator/callback",
			))
		})

		It("Should render a separate OAuth2Client for each slug-based clientID and for publishAPI", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			for _, c := range []struct{ clientID string }{
				{clientID: "kubeconfig-publish-api"},
				{clientID: "kubeconfig-plain"},
				{clientID: "kubeconfig-prod-eu"},
			} {
				crName := encoding.ToFnvLikeDex(c.clientID)
				oc := hec.KubernetesResource("OAuth2Client", "d8-user-authn", crName)
				Expect(oc.Exists()).To(BeTrue(), "OAuth2Client %s (CR %s) must exist", c.clientID, crName)
				Expect(oc.Field("id").String()).To(Equal(c.clientID))

				uris := []string{}
				for _, u := range oc.Field("redirectURIs").Array() {
					uris = append(uris, u.String())
				}
				Expect(uris).To(ConsistOf(
					"http://localhost:8000",
					"http://localhost:18000",
					"/device/callback",
				))
			}
		})
	})
})
