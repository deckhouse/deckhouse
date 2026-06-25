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

		It("must not render SPE and exception label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "monitoring-ping")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping").Exists()).To(BeFalse())
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

		It("must render a single merged SPE and pod-wide exception label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "monitoring-ping")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("monitoring-ping"))

			pseBase := f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping")
			Expect(pseBase.Exists()).To(BeTrue())
			Expect(pseBase.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(pseBase.Field("spec.securityContext.runAsUser.allowedValues").String()).To(MatchYAML(`
- 0
`))
			Expect(pseBase.Field("spec.securityContext.runAsNonRoot.allowedValue").Bool()).To(BeFalse())
			Expect(pseBase.Field("spec.securityContext.allowPrivilegeEscalation.allowedValue").Exists()).To(BeFalse(),
				"allowPrivilegeEscalation:true must not be whitelisted by the SPE; the init container is expected to set allowPrivilegeEscalation:false")

			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "monitoring-ping-root-init").Exists()).To(BeFalse())
		})

		It("must drop ALL capabilities on every container in the DaemonSet (restricted PSS compliance)", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "monitoring-ping")
			Expect(daemonSet.Exists()).To(BeTrue())

			initContainers := daemonSet.Field("spec.template.spec.initContainers").Array()
			Expect(initContainers).ToNot(BeEmpty())
			for _, c := range initContainers {
				name := c.Get("name").String()
				drops := c.Get("securityContext.capabilities.drop").Array()
				dropStrings := make([]string, 0, len(drops))
				for _, d := range drops {
					dropStrings = append(dropStrings, d.String())
				}
				Expect(dropStrings).To(ContainElement("ALL"),
					"initContainer %q must drop ALL capabilities under restricted PSS", name)
				Expect(c.Get("securityContext.allowPrivilegeEscalation").Bool()).To(BeFalse(),
					"initContainer %q must set allowPrivilegeEscalation:false", name)
				Expect(c.Get("securityContext.seccompProfile.type").String()).To(Equal("RuntimeDefault"),
					"initContainer %q must set seccompProfile.type=RuntimeDefault", name)
			}

			containers := daemonSet.Field("spec.template.spec.containers").Array()
			Expect(containers).ToNot(BeEmpty())
			for _, c := range containers {
				name := c.Get("name").String()
				drops := c.Get("securityContext.capabilities.drop").Array()
				dropStrings := make([]string, 0, len(drops))
				for _, d := range drops {
					dropStrings = append(dropStrings, d.String())
				}
				Expect(dropStrings).To(ContainElement("ALL"),
					"container %q must drop ALL capabilities under restricted PSS", name)
			}
		})
	})
})
