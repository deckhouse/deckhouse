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

var _ = Describe("Module :: monitoring-kubernetes-control-plane :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	Context("without admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["monitoring-kubernetes-control-plane", "control-plane-manager"]
discovery:
  clusterMasterCount: 0
  d8SpecificNodeCountByRole:
    master: 0
    system: 1
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("must not render SPE and exception label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "control-plane-proxy")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "control-plane-proxy").Exists()).To(BeFalse())
		})
	})

	Context("with admission-policy-engine-crd", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", `
enabledModules: ["monitoring-kubernetes-control-plane", "control-plane-manager", "admission-policy-engine", "admission-policy-engine-crd"]
discovery:
  clusterMasterCount: 0
  d8SpecificNodeCountByRole:
    master: 0
    system: 1
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("must render SPE and exception label", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			daemonSet := f.KubernetesResource("DaemonSet", "d8-monitoring", "control-plane-proxy")
			Expect(daemonSet.Exists()).To(BeTrue())
			Expect(daemonSet.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").String()).To(Equal("control-plane-proxy"))

			securityPolicyException := f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "control-plane-proxy")
			Expect(securityPolicyException.Exists()).To(BeTrue())
			Expect(securityPolicyException.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
		})

		It("must drop ALL capabilities on every container in control-plane-proxy DaemonSet", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ds := f.KubernetesResource("DaemonSet", "d8-monitoring", "control-plane-proxy")
			Expect(ds.Exists()).To(BeTrue())

			for _, c := range ds.Field("spec.template.spec.initContainers").Array() {
				name := c.Get("name").String()
				drops := c.Get("securityContext.capabilities.drop").Array()
				dropStrings := make([]string, 0, len(drops))
				for _, d := range drops {
					dropStrings = append(dropStrings, d.String())
				}
				Expect(dropStrings).To(ContainElement("ALL"),
					"initContainer %q must drop ALL capabilities under restricted PSS", name)
			}

			containers := ds.Field("spec.template.spec.containers").Array()
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
