/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

var _ = Describe("Module :: runtime-audit-engine :: helm template :: runtime-audit-engine ", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.25.0")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
	})
	Context("With Static mode set", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("runtimeAuditEngine", `
debugLogging: false
internal:
  webhookCertificate:
    ca: ABC
    crt: ABC
    key: ABC
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

			testD := hec.KubernetesResource("DaemonSet", "d8-runtime-audit-engine", "runtime-audit-engine")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`ephemeral-storage: 50Mi`))

			manVPA := hec.KubernetesResource("VerticalPodAutoscaler", "d8-runtime-audit-engine", "runtime-audit-engine")
			Expect(manVPA.Exists()).To(BeTrue())
			Expect(manVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Off"))
			Expect(manVPA.Field("spec.resourcePolicy.containerPolicies").Exists()).To(BeFalse())
		})
	})

	Context("With VPA mode set", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("runtimeAuditEngine", `
debugLogging: false
internal:
  webhookCertificate:
    ca: ABC
    crt: ABC
    key: ABC
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

			testD := hec.KubernetesResource("DaemonSet", "d8-runtime-audit-engine", "runtime-audit-engine")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`
ephemeral-storage: 50Mi
`))

			manVPA := hec.KubernetesResource("VerticalPodAutoscaler", "d8-runtime-audit-engine", "runtime-audit-engine")
			Expect(manVPA.Exists()).To(BeTrue())
			Expect(manVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(manVPA.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: falco
  controlledValues: RequestsAndLimits
  maxAllowed:
    cpu: 500m
    memory: 2048Mi
  minAllowed:
    cpu: 50m
    memory: 64Mi
- containerName: falcosidekick
  maxAllowed:
    cpu: 100m
    memory: 300Mi
  minAllowed:
    cpu: 5m
    memory: 10Mi
- containerName: rules-loader
  maxAllowed:
    cpu: 100m
    memory: 300Mi
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
