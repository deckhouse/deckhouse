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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValuesBootstrapped = `
clusterIsBootstrapped: true
enabledModules: ["vertical-pod-autoscaler", "registry-packages-proxy", "cert-manager"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
  ingressClass: nginx
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterUUID: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
`

const globalValuesNotBootstrapped = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler", "registry-packages-proxy", "cert-manager"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
  ingressClass: nginx
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  clusterUUID: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
`

const customCertificatePresent = `
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
`

var _ = Describe("Module :: registry-packages-proxy :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Cluster bootstrapped, https.mode = CustomCertificate", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
		})

		It("renders the ingress-tls-customcertificate secret in d8-cloud-instance-manager", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			created := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "ingress-tls-customcertificate")
			Expect(created.Exists()).To(BeTrue())
			Expect(created.Field("data").String()).To(Equal(`{"tls.crt":"Q1JUQ1JUQ1JU","tls.key":"S0VZS0VZS0VZ"}`))
		})
	})

	Context("Cluster not bootstrapped", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesNotBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
		})

		It("does not render the secret because the public ingress is not deployed yet", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			created := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "ingress-tls-customcertificate")
			Expect(created.Exists()).To(BeFalse())
		})
	})
})
