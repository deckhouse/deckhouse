/*
Copyright 2022 Flant JSC
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

var _ = Describe("Module :: monitoring-applications :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("monitoringApplications.internal.allowedApplications", `["rabbitmq", "redis"]`)
			f.ValuesSetFromYaml("monitoringApplications.internal.enabledApplicationsSummary", `["rabbitmq", "redis"]`)
			f.ValuesSetFromYaml("monitoringApplications.enabledApplications", `[]`)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ServiceMonitor", "d8-monitoring", "monitoring-applications").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "monitoring-applications-rabbitmq").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "monitoring-applications-redis").Exists()).To(BeTrue())

			Expect(f.KubernetesGlobalResource("GrafanaDashboardDefinition", "d8-applications-rabbitmq").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("GrafanaDashboardDefinition", "d8-applications-rabbitmq-legacy").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("GrafanaDashboardDefinition", "d8-applications-redis").Exists()).To(BeTrue())
		})
	})
})
