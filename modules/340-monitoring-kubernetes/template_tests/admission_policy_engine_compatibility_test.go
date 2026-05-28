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

const monitoringKubernetesCompatibilityValues = `
oomKillsExporterEnabled: true
internal:
  clusterDNSImplementation: coredns
  vpa:
    kubeStateMetricsMaxCPU: "115m"
    kubeStateMetricsMaxMemory: "180Mi"
`

var _ = Describe("Module :: monitoring-kubernetes :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	Context("without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["monitoring-kubernetes"]
discovery:
  d8SpecificNodeCountByRole:
    system: 1
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("monitoringKubernetes", monitoringKubernetesCompatibilityValues)
			f.HelmRender()
		})

		It("must not render SPE and exception labels", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			nodeExporterDaemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "node-exporter")
			Expect(nodeExporterDaemonSet.Exists()).To(BeTrue())
			Expect(nodeExporterDaemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "node-exporter").Exists()).To(BeFalse())

			oomKillsDaemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "oom-kills-exporter")
			Expect(oomKillsDaemonSet.Exists()).To(BeTrue())
			Expect(oomKillsDaemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "oom-kills-exporter").Exists()).To(BeFalse())
		})
	})

	Context("with admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["monitoring-kubernetes", "admission-policy-engine", "admission-policy-engine-crd"]
discovery:
  d8SpecificNodeCountByRole:
    system: 1
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("monitoringKubernetes", monitoringKubernetesCompatibilityValues)
			f.HelmRender()
		})

		It("must render SPE and exception labels for daemonsets requiring host access", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			nodeExporterDaemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "node-exporter")
			Expect(nodeExporterDaemonSet.Exists()).To(BeTrue())
			Expect(nodeExporterDaemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("node-exporter"))
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "node-exporter").Exists()).To(BeTrue())

			oomKillsDaemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "oom-kills-exporter")
			Expect(oomKillsDaemonSet.Exists()).To(BeTrue())
			Expect(oomKillsDaemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("oom-kills-exporter"))
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "oom-kills-exporter").Exists()).To(BeTrue())
		})

		It("must drop ALL capabilities on every container in node-exporter and oom-kills-exporter DaemonSets", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, dsName := range []string{"node-exporter", "oom-kills-exporter"} {
				ds := f.KubernetesResource("DaemonSet", "d8-monitoring", dsName)
				Expect(ds.Exists()).To(BeTrue(), "DaemonSet %q must exist", dsName)

				for _, c := range ds.Field("spec.template.spec.initContainers").Array() {
					name := c.Get("name").String()
					drops := c.Get("securityContext.capabilities.drop").Array()
					dropStrings := make([]string, 0, len(drops))
					for _, d := range drops {
						dropStrings = append(dropStrings, d.String())
					}
					Expect(dropStrings).To(ContainElement("ALL"),
						"DS %q initContainer %q must drop ALL capabilities under restricted PSS", dsName, name)
				}

				containers := ds.Field("spec.template.spec.containers").Array()
				Expect(containers).ToNot(BeEmpty(), "DS %q must have containers", dsName)
				for _, c := range containers {
					name := c.Get("name").String()
					drops := c.Get("securityContext.capabilities.drop").Array()
					dropStrings := make([]string, 0, len(drops))
					for _, d := range drops {
						dropStrings = append(dropStrings, d.String())
					}
					Expect(dropStrings).To(ContainElement("ALL"),
						"DS %q container %q must drop ALL capabilities under restricted PSS", dsName, name)
				}
			}
		})

		It("must keep kube-state-metrics without exception", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			kubeStateMetricsDeployment := f.KubernetesResource("Deployment", "d8-monitoring", "kube-state-metrics")
			Expect(kubeStateMetricsDeployment.Exists()).To(BeTrue())
			Expect(kubeStateMetricsDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "kube-state-metrics").Exists()).To(BeFalse())
		})
	})
})
