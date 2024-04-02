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

var _ = Describe("Module :: node-manager :: helm template :: standby node", func() {
	f := SetupHelmConfig(``)

	Context("Two NGs with standby", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.standbyNodeGroups", `[{name: standby-absolute, standby: 2, reserveCPU: "5500m", reserveMemory: "983Mi", taints: [{effect: NoExecute, key: ship-class, value: frigate}]}, {name: standby-percent, standby: 12, reserveCPU: "3400m", reserveMemory: 10Mi, taints: [{effect: NoExecute, key: ship-class, value: frigate}]}]`)
			f.ValuesSetFromYaml("nodeManager.internal.capiControllerManagerWebhookCert", `{ca: string, crt: string, key: string}`)
			f.ValuesSetFromYaml("nodeManager.internal.capsControllerManagerWebhookCert", `{ca: string, crt: string, key: string}`)
			f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{"master":1}`)
			f.ValuesSetFromYaml("global.clusterConfiguration", `apiVersion: deckhouse.io/v1
cloud:
  prefix: sandbox
  provider: vSphere
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Docker
kind: ClusterConfiguration
kubernetesVersion: "1.29"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
		})

		It("should render correctly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			da := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "standby-holder-standby-absolute")
			Expect(da.Exists()).To(BeTrue())
			Expect(da.Field("spec.replicas").String()).To(Equal("2"))
			Expect(da.Field("spec.template.spec.priorityClassName").String()).To(Equal("standby"))
			Expect(da.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("5500m"))
			Expect(da.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("983Mi"))
			Expect(da.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: ship-class
  value: frigate
  effect: NoExecute
`))

			dp := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "standby-holder-standby-percent")
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.replicas").String()).To(Equal("12"))
			Expect(dp.Field("spec.template.spec.priorityClassName").String()).To(Equal("standby"))
			Expect(dp.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("3400m"))
			Expect(dp.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("10Mi"))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: ship-class
  value: frigate
  effect: NoExecute
`))
		})
	})
})
