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

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// A password reset by user-authn-controller puts the replaced hash into the password history, so the
// controller has to keep exactly the history Dex is configured with.
var _ = Describe("Module :: user-authn :: helm template :: controller password history", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.31.1")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "test")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "testcert")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "testkey")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "testCA")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
	})

	historyLimitEnv := func() string {
		deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller")
		Expect(deployment.Exists()).To(BeTrue())
		env := deployment.Field(`spec.template.spec.containers.#(name=="user-authn-controller").env.#(name=="PASSWORD_HISTORY_LIMIT")`)
		Expect(env.Exists()).To(BeTrue())
		return env.Get("value").String()
	}

	dexConfig := func() string {
		secret := hec.KubernetesResource("Secret", "d8-user-authn", "dex")
		Expect(secret.Exists()).To(BeTrue())
		raw, err := base64.StdEncoding.DecodeString(secret.Field("data.config\\.yaml").String())
		Expect(err).ToNot(HaveOccurred())
		return string(raw)
	}

	It("passes the history limit of the Dex password policy", func() {
		hec.ValuesSetFromYaml("userAuthn.passwordPolicy", `
complexityLevel: Fair
passwordHistoryLimit: 3
`)
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		Expect(dexConfig()).To(ContainSubstring("passwordHistoryLimit: 3\n"))
		Expect(historyLimitEnv()).To(Equal("3"))
	})

	It("keeps no history without a password policy, as Dex does", func() {
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		Expect(dexConfig()).ToNot(ContainSubstring("passwordPolicy:"))
		Expect(historyLimitEnv()).To(Equal("0"))
	})
})
