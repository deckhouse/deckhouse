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

const monitoringPingCompatibilityValues = `
internal:
  clusterTargets:
    - name: test-target
      ipAddress: 10.0.0.1
`

var _ = Describe("Module :: monitoring-ping :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	Context("without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["monitoring-ping", "monitoring-kubernetes"]
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("monitoringPing", monitoringPingCompatibilityValues)
			f.HelmRender()
		})

		It("must not render SPE pair and exception labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "monitoring-ping")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception/monitoring-ping-clean-node-exporter-stale").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping-root-init").Exists()).To(BeFalse())
		})
	})

	Context("with admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["monitoring-ping", "monitoring-kubernetes", "admission-policy-engine", "admission-policy-engine-crd"]
discovery:
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("monitoringPing", monitoringPingCompatibilityValues)
			f.HelmRender()
		})

		It("must render SPE pair and pod-wide + per-container exception labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "monitoring-ping")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("monitoring-ping"))
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception/monitoring-ping-clean-node-exporter-stale").String()).To(Equal("monitoring-ping-root-init"))

			pseBase := f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping")
			Expect(pseBase.Exists()).To(BeTrue())
			Expect(pseBase.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(pseBase.Field("spec.securityContext.runAsUser").Exists()).To(BeFalse())
			Expect(pseBase.Field("spec.securityContext.runAsNonRoot").Exists()).To(BeFalse())

			pseInit := f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping-root-init")
			Expect(pseInit.Exists()).To(BeTrue())
			Expect(pseInit.Field("spec.securityContext.runAsUser.allowedValues").String()).To(MatchYAML(`
- 0
`))
			Expect(pseInit.Field("spec.securityContext.runAsNonRoot.allowedValue").Bool()).To(BeFalse())
		})
	})
})
