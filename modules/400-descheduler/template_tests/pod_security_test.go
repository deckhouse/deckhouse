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

var _ = Describe("Module :: descheduler :: helm template :: pod security", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYamlWithOpenAPIDefaults("descheduler", `
internal:
  deschedulers:
  - name: test
    strategies:
      lowNodeUtilization:
        enabled: true
        thresholds:
          cpu: 10
        targetThresholds:
          cpu: 40
`)
		f.HelmRender()
	})

	It("Namespace must opt into enforcement of the restricted policy", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		namespace := f.KubernetesGlobalResource("Namespace", "d8-descheduler")
		Expect(namespace.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
		Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/pod-policy`).String()).To(Equal("restricted"))
		Expect(namespace.Field(`metadata.labels.security\.deckhouse\.io/enable-security-policy-check`).String()).To(Equal("true"))
	})

	It("descheduler must pass the restricted policy without an exception", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		deployment := f.KubernetesResource("Deployment", "d8-descheduler", "descheduler")
		Expect(deployment.Exists()).To(BeTrue())
		podTemplate := deployment.Field("spec.template")
		Expect(podTemplate.Get(`metadata.labels.security\.deckhouse\.io/security-policy-exception`).Exists()).To(BeFalse())
		Expect(PodRestrictedViolations(podTemplate)).To(BeEmpty())
	})
})
