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

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: userAuthn :: helm template :: custom-certificate", func() {
	const globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "user-authn", "cert-manager"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  kubernetesCA: plainstring
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("userAuthn.https", `{"mode":"CustomCertificate"}`)
			f.ValuesSetFromYaml("userAuthn.internal.dexTLS", `{"crt":"plainstring","key":"plainstring"}`)
			f.ValuesSetFromYaml("userAuthn.internal.customCertificateData", `{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`)
			f.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeFalse())
		})

	})

	Context("Default with PublishAPI", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("userAuthn.https", `{"mode":"CustomCertificate"}`)
			f.ValuesSetFromYaml("userAuthn.internal.dexTLS", `{"crt":"plainstring","key":"plainstring"}`)
			f.ValuesSetFromYaml("userAuthn.internal.customCertificateData", `{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`)
			f.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
			f.ValuesSet("userAuthn.publishAPI.enabled", true)
			f.ValuesSet("userAuthn.publishAPI.https.mode", "Global")
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
		})

	})

})
