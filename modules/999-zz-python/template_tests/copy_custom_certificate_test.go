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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
clusterIsBootstrapped: false
enabledModules: ["vertical-pod-autoscaler-crd", "deckhouse-web"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

var _ = Describe("Module :: zz-python :: helm template :: ConfigMap", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("zz-python", `{}`)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdCM := f.KubernetesResource("Secret", "d8-system", "zz-python")
			Expect(createdCM.Exists()).To(BeTrue())
			Expect(createdCM.Field("data").String()).To(Equal(`{"valuesStatement":"default_statement"}`))
		})

	})

	Context("With config", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("zz-python", `
statement: "custom_statement"
array: [2, 1]
`)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdCM := f.KubernetesResource("Secret", "d8-system", "zz-python")
			Expect(createdCM.Exists()).To(BeTrue())
			Expect(createdCM.Field("data").String()).To(Equal(`{"valuesStatement":"ARRAY IS HERE", "configStatement": "custom_statement", "array": "[2, 1]"}`))
		})

	})

})
