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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: dex-config", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.21.1")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "test")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "testcert")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "testkey")

		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
	})

	Context("Without Users and Providers", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})

		It("Should create dex config with enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).To(BeEmpty())
			Expect(config).To(ContainSubstring("enablePasswordDB: true"))

			Expect(gjson.GetBytes(configBytes, "twoFactorAuthn").String()).To(Equal(""))
		})
	})

	Context("With 2FA", func() {
		BeforeEach(func() {
			hec.ValuesSet("userAuthn.staticUsers2FA.enabled", true)
			hec.ValuesSet("userAuthn.staticUsers2FA.issuerName", "Deckhouse (Alpha)")
			hec.HelmRender()
		})

		It("Should add 2FA settings", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)

			jsonConfig, err := yaml.YAMLToJSON(configBytes)
			Expect(err).ToNot(HaveOccurred())

			Expect(gjson.GetBytes(jsonConfig, "twoFactorAuthn.issuer").String()).To(Equal("Deckhouse (Alpha)"))
			Expect(gjson.GetBytes(jsonConfig, "twoFactorAuthn.connectors").String()).To(Equal(`["local"]`))
		})
	})

	Context("Without Users but with Providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: gitlabID
  displayName: gitlabName
  type: Gitlab
  gitlab:
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
    groups:
    - Admins
    - Everyone`)
			hec.HelmRender()
		})

		It("Should create dex config without enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).To(BeEmpty())
			Expect(config).NotTo(ContainSubstring("enablePasswordDB: true"))
		})
	})

	Context("With Users but without providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexUsersCRDs", `
- encodedName: encodedUser
  name: userName
  spec:
    email: user@example.com
    groups:
    - Everyone
    password: userPassword
- encodedName: encodedAdmin
  name: adminName
  spec:
    email: adminTest@example.com
    groups:
    - Everyone
    - Admins
    password: adminPassword
`)
			hec.HelmRender()
		})

		It("Should create dex config with enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).NotTo(BeEmpty())
			Expect(config).To(ContainSubstring("enablePasswordDB: true"))
		})
	})

	Context("With Users and providers", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexUsersCRDs", `
- encodedName: encodedUser
  name: userName
  spec:
    email: user@example.com
    groups:
    - Everyone
    password: userPassword
- encodedName: encodedAdmin
  name: adminName
  spec:
    email: adminTest@example.com
    groups:
    - Everyone
    - Admins
    password: adminPassword
`)
			hec.ValuesSetFromYaml("userAuthn.internal.providers", `
- id: gitlabID
  displayName: gitlabName
  type: Gitlab
  gitlab:
    clientID: clientID
    clientSecret: secret
    baseURL: https://example.com
    groups:
    - Admins
    - Everyone`)
			hec.HelmRender()
		})

		It("Should create dex config with enablePasswordDB", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			dexConfig := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
			b64Config := dexConfig.Field("data.config\\.yaml").String()
			configBytes, _ := base64.StdEncoding.DecodeString(b64Config)
			config := string(configBytes)
			usersExist := hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()

			Expect(usersExist).NotTo(BeEmpty())
			Expect(config).To(ContainSubstring("enablePasswordDB: true"))
		})
	})
})
