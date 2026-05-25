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

const logShipperCompatibilityValues = `
debug: false
internal:
  activated: true
resourcesRequests:
  mode: Static
  static:
    cpu: 5m
    memory: 4Mi
  vpa:
    cpu:
      max: 500m
      min: 50m
    memory:
      max: 2048Mi
      min: 64Mi
    mode: Initial
`

var _ = Describe("Module :: log-shipper :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig("")

	Context("without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["log-shipper", "vertical-pod-autoscaler"]
discovery:
  kubernetesVersion: "1.31.14"
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("logShipper", logShipperCompatibilityValues)
			f.HelmRender()
		})

		It("must render restricted namespace label and skip security-check label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-log-shipper")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/pod-policy").String()).To(Equal("restricted"))
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").Exists()).To(BeFalse())
		})

		It("must not render SPE and exception label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-log-shipper", "log-shipper-agent")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-log-shipper", "log-shipper-agent").Exists()).To(BeFalse())
		})
	})

	Context("with admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["log-shipper", "vertical-pod-autoscaler", "admission-policy-engine", "admission-policy-engine-crd"]
discovery:
  kubernetesVersion: "1.31.14"
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("logShipper", logShipperCompatibilityValues)
			f.HelmRender()
		})

		It("must render restricted namespace label and security-check label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-log-shipper")
			Expect(namespace.Exists()).To(BeTrue())
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/pod-policy").String()).To(Equal("restricted"))
			Expect(namespace.Field("metadata.labels.security\\.deckhouse\\.io/enable-security-policy-check").String()).To(Equal("true"))
		})

		It("must render SPE and exception label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-log-shipper", "log-shipper-agent")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("log-shipper-agent"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-log-shipper", "log-shipper-agent")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.securityContext.runAsUser.allowedValues").String()).To(MatchYAML(`
- 0
`))
			Expect(securityPolicyException.Field("spec.volumes.types.allowedValues").String()).To(MatchYAML(`
- hostPath
`))
		})
	})
})
