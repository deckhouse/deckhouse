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

const dexClientAppSecret = `Secret with
New string
`

func assertCreateConfig(hec *Config) []byte {
	Expect(hec.KubernetesResource("Secret", "d8-user-authn", "kubeconfig-generator").Exists()).To(BeTrue())

	kgConfig := hec.KubernetesResource("Secret", "d8-user-authn", "kubeconfig-generator")
	b64ConfigStr := []byte(kgConfig.Field("data.config\\.yaml").String())

	configStr, err := base64.StdEncoding.DecodeString(string(b64ConfigStr))
	Expect(err).ToNot(HaveOccurred())

	var b64Config map[string]interface{}
	err = yaml.Unmarshal(configStr, &b64Config)
	Expect(err).ToNot(HaveOccurred())

	jsonBytes, err := json.Marshal(b64Config)
	Expect(err).ToNot(HaveOccurred())

	return jsonBytes
}

func assertDefaultCluster(cl gjson.Result) {
	Expect(cl.Get("client_id").String()).To(Equal("kubeconfig-generator"))
	Expect(cl.Get("client_secret").String()).To(Equal(dexClientAppSecret))
	Expect(cl.Get("issuer").String()).To(Equal("https://dex.example.com/"))
	Expect(cl.Get("k8s_master_uri").String()).To(Equal("https://api.example.com"))
	Expect(cl.Get("name").String()).To(Equal("api.example.com"))
	Expect(cl.Get("redirect_uri").String()).To(Equal("https://kubeconfig.example.com/callback/"))
	Expect(cl.Get("short_description").String()).To(Equal("https://api.example.com"))
	Expect(cl.Get("scopes.0").String()).To(Equal("audience:server:client_id:kubernetes"))
}

var _ = Describe("Module :: user-authn :: helm template :: kubeconfig-generator-config", func() {
	hec := SetupHelmConfig("")

	const k8sCa = `---Certificate k8s--
Multiline
`
	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
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

	Context("Default config", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.publishAPI.enabled", true)
			hec.ValuesSet("userAuthn.publishAPI.addKubeconfigGeneratorEntry", true)

			hec.HelmRender()
		})

		It("Should create kubeconfig generator config", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			jsonBytes := assertCreateConfig(hec)

			Expect(gjson.GetBytes(jsonBytes, "trusted_root_ca").String()).To(Equal(k8sCa))
			Expect(gjson.GetBytes(jsonBytes, "listen").String()).To(Equal("http://0.0.0.0:5555"))
			Expect(gjson.GetBytes(jsonBytes, "logo_uri").String()).To(Equal("https://kubernetes.io/images/favicon.png"))
			Expect(gjson.GetBytes(jsonBytes, "web_path_prefix").String()).To(Equal("/"))
			Expect(gjson.GetBytes(jsonBytes, "debug").String()).To(Equal("false"))
			Expect(gjson.GetBytes(jsonBytes, "kubectl_version").String()).To(Equal("v1.15.6"))

			clusters := gjson.GetBytes(jsonBytes, "clusters").Array()

			Expect(len(clusters)).To(Equal(1))

			assertDefaultCluster(clusters[0])
		})

		Context("With discoveredDexCA", func() {
			BeforeEach(func() {
				hec.ValuesSet("userAuthn.publishAPI.enabled", true)
				hec.ValuesSet("userAuthn.publishAPI.addKubeconfigGeneratorEntry", true)
				hec.ValuesSet("userAuthn.internal.discoveredDexCA", "discoveredDexCAText")
				hec.HelmRender()
			})

			It("Should add idp_ca_pem setting", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				jsonBytes := assertCreateConfig(hec)

				Expect(gjson.GetBytes(jsonBytes, "idp_ca_pem").String()).To(Equal("discoveredDexCAText\n"))
			})
		})

		Context("Publish API", func() {
			JustBeforeEach(func() {
				hec.ValuesSet("userAuthn.publishAPI.enabled", true)
				hec.ValuesSet("userAuthn.publishAPI.addKubeconfigGeneratorEntry", true)
				hec.HelmRender()
			})

			Context("With https mode 'Global'", func() {
				BeforeEach(func() {
					hec.ValuesSet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA", "test_cert")
					hec.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
				})

				It("Should add k8s_ca_pem param with publishedAPIKubeconfigGeneratorMasterCA for first cluster", func() {
					Expect(hec.RenderError).ToNot(HaveOccurred())

					jsonBytes := assertCreateConfig(hec)

					Expect(gjson.GetBytes(jsonBytes, "clusters.0.k8s_ca_pem").String()).To(Equal("test_cert"))
				})
			})
		})

		Context("Additional access", func() {
			apis := []struct {
				id        string
				masterCA  string
				masterURI string
				desc      string
			}{
				{
					id:        "first",
					masterCA:  "caforfirst",
					masterURI: "https://first.master",
					desc:      "desc_first",
				},
				{
					id:        "second",
					masterCA:  "",
					masterURI: "https://second.master",
					desc:      "desc_first",
				},
			}

			JustBeforeEach(func() {
				encodedNames := make([]string, 0)
				for i, a := range apis {
					hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.id", i), a.id)
					hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.masterCA", i), a.masterCA)
					hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.masterURI", i), a.masterURI)
					hec.ValuesSet(fmt.Sprintf("userAuthn.kubeconfigGenerator.%v.description", i), a.desc)

					name := encoding.ToFnvLikeDex(fmt.Sprintf("kubeconfig-generator-%d", i))
					encodedNames = append(encodedNames, name)
				}

				hec.ValuesSet("userAuthn.internal.kubeconfigEncodedNames", encodedNames)

				hec.HelmRender()
			})

			It("Should render all additional clusters", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				jsonBytes := assertCreateConfig(hec)
				clusters := gjson.GetBytes(jsonBytes, "clusters").Array()

				Expect(len(clusters)).To(Equal(3))

				assertDefaultCluster(clusters[0])
				for i, a := range apis {
					cl := clusters[i+1]
					Expect(cl.Get("client_id").String()).To(Equal(fmt.Sprintf("kubeconfig-generator-%v", i)))
					Expect(cl.Get("client_secret").String()).To(Equal(dexClientAppSecret))
					Expect(cl.Get("issuer").String()).To(Equal("https://dex.example.com/"))
					Expect(cl.Get("k8s_master_uri").String()).To(Equal(a.masterURI))
					Expect(cl.Get("name").String()).To(Equal(fmt.Sprintf(a.id)))
					Expect(cl.Get("redirect_uri").String()).To(Equal(fmt.Sprintf("https://kubeconfig.example.com/callback/%v", i)))
					Expect(cl.Get("short_description").String()).To(Equal(a.desc))
					Expect(cl.Get("scopes.0").String()).To(Equal("audience:server:client_id:kubernetes"))
				}
			})

			It("Should render k8s_ca_pem param as masterCA if masterCA is present", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				jsonBytes := assertCreateConfig(hec)

				Expect(gjson.GetBytes(jsonBytes, "clusters.1.k8s_ca_pem").String()).To(Equal(apis[0].masterCA))
			})

			It("Should render k8s_ca_pem param as kubernetesCA if masterCA is not present", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				jsonBytes := assertCreateConfig(hec)

				Expect(gjson.GetBytes(jsonBytes, "clusters.2.k8s_ca_pem").String()).To(Equal(k8sCa))
			})
		})
	})
})
