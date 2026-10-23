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
	"crypto/sha256"
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

// kubernetesOAuth2ClientName is encoding.ToFnvLikeDex("kubernetes").
const kubernetesOAuth2ClientName = "nn2wezlsnzsxizltzpzjzzeeeirsk"

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

	Context("Without peers from the module configuration", func() {
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
    applications:
    - domain: authenticator.example.com
      ingressClassName: nginx
      ingressSecretName: test
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"test@d8-test":
  name: "test-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-test"
  secretTruncated: false
  secretHash: ""
  ingressNames:
    "0":
      name: "test-dex-authenticator"
      truncated: false
      hash: ""
`)
			hec.HelmRender()
		})
		// user-authn-controller keeps the client and takes the DexAuthenticator peers from the
		// DexAuthenticators themselves.
		It("Should leave the kubernetes OAuth2Client to the controller with no static peers", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", kubernetesOAuth2ClientName).Exists()).To(BeFalse())

			cm := hec.KubernetesResource("ConfigMap", "d8-user-authn", "kubernetes-oauth2client")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.trustedPeers").String()).To(Equal("[]"))
		})
	})

	Context("Full scenario: dex-authenticator + publishAPI + kubeconfigGenerator", func() {
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
    applications:
    - domain: with-access.example.com
      ingressClassName: nginx
      ingressSecretName: cert-1
- name: no-access
  encodedName: encNoAccess
  namespace: d8-no-access
  credentials:
    appDexSecret: s2
    cookieSecret: c2
  spec:
    applications:
    - domain: no-access.example.com
      ingressClassName: nginx
      ingressSecretName: cert-2
`)
			hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", `
"with-access@d8-with-access":
  name: "with-access-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-with-access"
  secretTruncated: false
  secretHash: ""
  ingressNames:
    "0":
      name: "with-access-dex-authenticator"
      truncated: false
      hash: ""
"no-access@d8-no-access":
  name: "no-access-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-no-access"
  secretTruncated: false
  secretHash: ""
  ingressNames:
    "0":
      name: "no-access-dex-authenticator"
      truncated: false
      hash: ""
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

		It("Should pass the static peers in order and the secret checksum to the controller", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("OAuth2Client", "d8-user-authn", kubernetesOAuth2ClientName).Exists()).To(BeFalse())

			cm := hec.KubernetesResource("ConfigMap", "d8-user-authn", "kubernetes-oauth2client")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.trustedPeers").String()).To(MatchJSON(`[
				"kubeconfig-generator",
				"kubeconfig-publish-api",
				"kubeconfig-generator-0",
				"kubeconfig-plain",
				"kubeconfig-generator-1",
				"kubeconfig-prod-eu"
			]`))
			sum := sha256.Sum256([]byte("plainstring"))
			Expect(cm.Field("metadata.annotations.checksum/secret").String()).To(Equal(hex.EncodeToString(sum[:])))
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

		// The kubeconfig-generator web UI is gone, but its dex clients outlive it: every
		// kubeconfig the UI ever issued carries one of these client ids and refreshes its
		// token through them. They are scheduled for removal one release later, so until
		// then dropping templates/kubeconfig-generator/ — now down to two files — would
		// lock those users out at both login and refresh. Pin the clients and the CLI
		// callbacks they must keep accepting.
		It("Should keep the legacy kubeconfig-generator OAuth2Clients for already issued kubeconfigs", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			for _, clientID := range []string{
				"kubeconfig-generator",
				"kubeconfig-generator-0",
				"kubeconfig-generator-1",
			} {
				crName := encoding.ToFnvLikeDex(clientID)
				oc := hec.KubernetesResource("OAuth2Client", "d8-user-authn", crName)
				Expect(oc.Exists()).To(BeTrue(),
					"legacy OAuth2Client %s (CR %s) must survive the web UI removal", clientID, crName)
				Expect(oc.Field("id").String()).To(Equal(clientID))

				uris := []string{}
				for _, u := range oc.Field("redirectURIs").Array() {
					uris = append(uris, u.String())
				}
				// kubelogin and the device flow are the only ways these kubeconfigs can
				// still authenticate: the web UI callback went away with the hostname.
				Expect(uris).To(ConsistOf(
					"http://localhost:8000",
					"http://localhost:18000",
					"/device/callback",
				))
			}
		})
	})
})
