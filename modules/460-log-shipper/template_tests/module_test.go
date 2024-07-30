/*
Copyright 2023 Flant JSC

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

var _ = Describe("Module :: log-shipper :: helm template :: log-shipper ", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.27.0")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
	})
	Context("With Static mode set", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("logShipper", `
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
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			testD := hec.KubernetesResource("DaemonSet", "d8-log-shipper", "log-shipper-agent")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`ephemeral-storage: 1024Mi`))

			manVPA := hec.KubernetesResource("VerticalPodAutoscaler", "d8-log-shipper", "log-shipper-agent")
			Expect(manVPA.Exists()).To(BeTrue())
			Expect(manVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Off"))
			Expect(manVPA.Field("spec.resourcePolicy.containerPolicies").Exists()).To(BeFalse())
		})
	})

	Context("With VPA mode set", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("logShipper", `
debug: false
internal:
  activated: true
resourcesRequests:
  mode: VPA
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
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			testD := hec.KubernetesResource("DaemonSet", "d8-log-shipper", "log-shipper-agent")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`
cpu: 50m
ephemeral-storage: 1024Mi
memory: 64Mi
`))

			manVPA := hec.KubernetesResource("VerticalPodAutoscaler", "d8-log-shipper", "log-shipper-agent")
			Expect(manVPA.Exists()).To(BeTrue())
			Expect(manVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(manVPA.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: vector
  controlledValues: RequestsAndLimits
  maxAllowed:
    cpu: 500m
    memory: 2048Mi
  minAllowed:
    cpu: 50m
    memory: 64Mi
- containerName: vector-reloader
  maxAllowed:
    cpu: 20m
    memory: 25Mi
  minAllowed:
    cpu: 10m
    memory: 25Mi
- containerName: kube-rbac-proxy
  maxAllowed:
    cpu: 20m
    memory: 25Mi
  minAllowed:
    cpu: 10m
    memory: 25Mi`))
		})
	})
})
