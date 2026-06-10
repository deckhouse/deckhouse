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

var _ = Describe("Module :: operator-prometheus :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	Context("without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["operator-prometheus"]
modules:
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("must render restricted namespace label and skip security-check label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-operator-prometheus")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/pod-policy").String()).To(Equal("restricted"))
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").Exists()).To(BeFalse())
		})

		It("must not render SecurityPolicyException resources or exception pod labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			operatorDeployment := f.KubernetesResource("Deployment", "d8-operator-prometheus", "prometheus-operator")
			Expect(operatorDeployment.Exists()).To(BeTrue())
			Expect(operatorDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-operator-prometheus", "prometheus-operator").Exists()).To(BeFalse())
		})
	})

	Context("with admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["operator-prometheus", "admission-policy-engine", "admission-policy-engine-crd"]
modules:
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("must render restricted namespace label and security-check label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-operator-prometheus")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/pod-policy").String()).To(Equal("restricted"))
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").String()).To(Equal("true"))
		})

		It("must keep workload without exceptions", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			operatorDeployment := f.KubernetesResource("Deployment", "d8-operator-prometheus", "prometheus-operator")
			Expect(operatorDeployment.Exists()).To(BeTrue())
			Expect(operatorDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-operator-prometheus", "prometheus-operator").Exists()).To(BeFalse())
		})
	})
})
