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
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

const newSecretName = "kubernetes-dex-client-configuration"

func unmarshalNewSecret(hec *Config) []byte {
	resource := hec.KubernetesResource("Secret", "d8-user-authn", newSecretName)
	Expect(resource.Exists()).To(BeTrue())

	b64ConfigStr := []byte(resource.Field("data.config\\.yaml").String())
	configStr, err := base64.StdEncoding.DecodeString(string(b64ConfigStr))
	Expect(err).ToNot(HaveOccurred())

	var parsed map[string]any
	err = yaml.Unmarshal(configStr, &parsed)
	Expect(err).ToNot(HaveOccurred())

	jsonBytes, err := json.Marshal(parsed)
	Expect(err).ToNot(HaveOccurred())

	return jsonBytes
}

var _ = Describe("Module :: user-authn :: helm template :: kubernetes-dex-client-configuration", func() {
	hec := SetupHelmConfig("")

	const (
		k8sCa             = "---k8s CA---\nmultiline\n"
		dexClientAppSecret = "client-app-secret-value"
	)

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", k8sCa)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", dexClientAppSecret)
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "do not use, but set")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "do not use, but set")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", k8sCa)

		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "do not use, but set")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "do not use, but set")
	})

	Context("Neither publishAPI nor kubeconfigGenerator", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.publishAPI.enabled", false)
			hec.HelmRender()
		})

		It("Should NOT render the new secret", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			resource := hec.KubernetesResource("Secret", "d8-user-authn", newSecretName)
			Expect(resource.Exists()).To(BeFalse())
		})
	})

	Context("publishAPI only", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.publishAPI.enabled", true)
			hec.ValuesSet("userAuthn.internal.publishAPI.addKubeconfigGeneratorEntry", true)
			hec.ValuesSet("userAuthn.internal.publishAPI.publishedAPIKubeconfigGeneratorMasterCA", "publish-api-ca")
			hec.ValuesSet("userAuthn.internal.discoveredDexCA", "discoveredDexCAText")
			hec.ValuesSet("userAuthn.internal.kubeconfigPublishAPIEncodedName",
				encoding.ToFnvLikeDex("kubeconfig-publish-api"))
			hec.HelmRender()
		})

		It("Should render exactly one publishAPI cluster entry with idpCA", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonBytes := unmarshalNewSecret(hec)

			Expect(gjson.GetBytes(jsonBytes, "idpCA").String()).To(Equal("discoveredDexCAText\n"))

			clusters := gjson.GetBytes(jsonBytes, "clusters").Array()
			Expect(clusters).To(HaveLen(1))

			cl := clusters[0]
			Expect(cl.Get("id").String()).To(Equal("api.example.com"))
			Expect(cl.Get("name").String()).To(Equal("api.example.com"))
			Expect(cl.Get("masterURI").String()).To(Equal("https://api.example.com"))
			Expect(cl.Get("masterCA").String()).To(Equal("publish-api-ca"))
			Expect(cl.Get("clientID").String()).To(Equal("kubeconfig-publish-api"))
			Expect(cl.Get("clientSecret").String()).To(Equal(dexClientAppSecret))
			Expect(cl.Get("issuer").String()).To(Equal("https://dex.example.com/"))
		})

		It("Should not include any legacy snake_case fields", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonBytes := unmarshalNewSecret(hec)

			for _, legacyKey := range []string{
				"listen", "logo_uri", "web_path_prefix", "debug",
				"trusted_root_ca", "idp_ca_pem", "kubectl_version",
			} {
				Expect(gjson.GetBytes(jsonBytes, legacyKey).Exists()).
					To(BeFalse(), "legacy top-level field %q should be absent", legacyKey)
			}

			cl := gjson.GetBytes(jsonBytes, "clusters.0")
			for _, legacyKey := range []string{
				"client_id", "client_secret", "k8s_master_uri",
				"k8s_ca_pem", "redirect_uri", "short_description", "scopes",
			} {
				Expect(cl.Get(legacyKey).Exists()).
					To(BeFalse(), "legacy cluster field %q should be absent", legacyKey)
			}
		})
	})

	Context("kubeconfigGenerator only", func() {
		apis := []struct {
			id        string
			masterCA  string
			masterURI string
		}{
			{id: "first", masterCA: "ca-for-first", masterURI: "https://first.master"},
			{id: "second", masterCA: "", masterURI: "https://second.master"},
		}

		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.publishAPI.enabled", false)
			encodedNames := make([]string, 0, len(apis))
			clientEncodedNames := make([]string, 0, len(apis))
			for i, a := range apis {
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.id", i), a.id)
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.masterCA", i), a.masterCA)
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.masterURI", i), a.masterURI)
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.description", i), "desc")
				encodedNames = append(encodedNames, encoding.ToFnvLikeDex(fmt.Sprintf("kubeconfig-generator-%d", i)))
				clientEncodedNames = append(clientEncodedNames, encoding.ToFnvLikeDex(fmt.Sprintf("kubeconfig-%s", a.id)))
			}
			hec.ValuesSet("userAuthn.internal.kubeconfigEncodedNames", encodedNames)
			hec.ValuesSet("userAuthn.internal.kubeconfigClientEncodedNames", clientEncodedNames)
			hec.HelmRender()
		})

		It("Should render one entry per kubeconfigGenerator item, no publishAPI entry", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonBytes := unmarshalNewSecret(hec)

			clusters := gjson.GetBytes(jsonBytes, "clusters").Array()
			Expect(clusters).To(HaveLen(len(apis)))

			for i, a := range apis {
				cl := clusters[i]
				Expect(cl.Get("id").String()).To(Equal(a.id))
				Expect(cl.Get("name").String()).To(Equal(a.id))
				Expect(cl.Get("masterURI").String()).To(Equal(a.masterURI))
				Expect(cl.Get("clientID").String()).To(Equal(fmt.Sprintf("kubeconfig-%s", a.id)))
				Expect(cl.Get("clientSecret").String()).To(Equal(dexClientAppSecret))
				Expect(cl.Get("issuer").String()).To(Equal("https://dex.example.com/"))
			}

			Expect(clusters[0].Get("masterCA").String()).To(Equal(apis[0].masterCA))
			Expect(clusters[1].Get("masterCA").String()).To(Equal(k8sCa))
		})
	})

	Context("clientID slugification", func() {
		slugCases := []struct {
			id       string
			clientID string
		}{
			{id: "admin@bastion", clientID: "kubeconfig-admin-at-bastion"},
			{id: "prod:eu-west", clientID: "kubeconfig-prod-eu-west"},
			{id: "plain.with.dots-and_underscores", clientID: "kubeconfig-plain.with.dots-and_underscores"},
		}

		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.publishAPI.enabled", false)
			encodedNames := make([]string, 0, len(slugCases))
			clientEncodedNames := make([]string, 0, len(slugCases))
			for i, c := range slugCases {
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.id", i), c.id)
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.masterURI", i), "https://m")
				hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.description", i), "d")
				encodedNames = append(encodedNames, encoding.ToFnvLikeDex(fmt.Sprintf("kubeconfig-generator-%d", i)))
				clientEncodedNames = append(clientEncodedNames, encoding.ToFnvLikeDex(c.clientID))
			}
			hec.ValuesSet("userAuthn.internal.kubeconfigEncodedNames", encodedNames)
			hec.ValuesSet("userAuthn.internal.kubeconfigClientEncodedNames", clientEncodedNames)
			hec.HelmRender()
		})

		It("Should slugify '@' to '-at-' and ':' to '-'", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonBytes := unmarshalNewSecret(hec)

			clusters := gjson.GetBytes(jsonBytes, "clusters").Array()
			Expect(clusters).To(HaveLen(len(slugCases)))

			for i, c := range slugCases {
				Expect(clusters[i].Get("clientID").String()).To(Equal(c.clientID))
				Expect(clusters[i].Get("id").String()).To(Equal(c.id))
			}
		})

		It("Should include both legacy and new trustedPeers in OAuth2Client", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			oauth2Client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "nn2wezlsnzsxizltzpzjzzeeeirsk")
			Expect(oauth2Client.Exists()).To(BeTrue())

			peers := oauth2Client.Field("trustedPeers").Array()
			peerSet := make(map[string]bool, len(peers))
			for _, p := range peers {
				peerSet[p.String()] = true
			}

			for i, c := range slugCases {
				Expect(peerSet[fmt.Sprintf("kubeconfig-generator-%d", i)]).
					To(BeTrue(), "legacy trustedPeer kubeconfig-generator-%d must remain in phase 1", i)
				Expect(peerSet[c.clientID]).
					To(BeTrue(), "new trustedPeer %q must be added", c.clientID)
			}
		})

		It("Should create a separate OAuth2Client for every slug-based clientID", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			for _, c := range slugCases {
				crName := encoding.ToFnvLikeDex(c.clientID)
				oc := hec.KubernetesResource("OAuth2Client", "d8-user-authn", crName)
				Expect(oc.Exists()).To(BeTrue(), "OAuth2Client for %q (CR %q) must exist", c.clientID, crName)
				Expect(oc.Field("id").String()).To(Equal(c.clientID))
				Expect(oc.Field("name").String()).To(Equal(c.clientID))
				Expect(oc.Field("secret").String()).To(Equal(dexClientAppSecret))

				uris := []string{}
				for _, u := range oc.Field("redirectURIs").Array() {
					uris = append(uris, u.String())
				}
				Expect(uris).To(ContainElement("/device/callback"))
				Expect(uris).NotTo(ContainElement(ContainSubstring("/callback/")),
					"new slug-based OAuth2Client must NOT expose the legacy kubeconfig UI redirect_uri")
			}
		})
	})

	Context("publishAPI and kubeconfigGenerator together", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.internal.publishAPI.enabled", true)
			hec.ValuesSet("userAuthn.internal.publishAPI.addKubeconfigGeneratorEntry", true)
			hec.ValuesSet("userAuthn.internal.publishAPI.publishedAPIKubeconfigGeneratorMasterCA", "publish-api-ca")

			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.id", "extra")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.masterURI", "https://extra.master")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.masterCA", "extra-ca")
			hec.ValuesSet("userAuthn.kubeconfigGenerator.0.description", "extra")
			hec.ValuesSet("userAuthn.internal.kubeconfigEncodedNames",
				[]string{encoding.ToFnvLikeDex("kubeconfig-generator-0")})
			hec.ValuesSet("userAuthn.internal.kubeconfigClientEncodedNames",
				[]string{encoding.ToFnvLikeDex("kubeconfig-extra")})

			hec.HelmRender()
		})

		It("Should render publishAPI entry first, kubeconfigGenerator entries after", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			jsonBytes := unmarshalNewSecret(hec)

			clusters := gjson.GetBytes(jsonBytes, "clusters").Array()
			Expect(clusters).To(HaveLen(2))

			Expect(clusters[0].Get("id").String()).To(Equal("api.example.com"))
			Expect(clusters[0].Get("masterCA").String()).To(Equal("publish-api-ca"))

			Expect(clusters[1].Get("id").String()).To(Equal("extra"))
			Expect(clusters[1].Get("masterCA").String()).To(Equal("extra-ca"))
		})
	})
})
