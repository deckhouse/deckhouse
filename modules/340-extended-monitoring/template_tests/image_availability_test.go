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

func checkImageAvailabilityObjects(hec *HelmConfig, exist bool) {
	matcher := BeFalse()
	if exist {
		matcher = BeTrue()
	}

	Expect(hec.KubernetesResource("Deployment", "d8-monitoring", "image-availability-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "image-availability-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-monitoring", "image-availability-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("ServiceAccount", "d8-monitoring", "image-availability-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("PrometheusRule", "d8-monitoring", "extended-monitoring-image-checks").Exists()).To(matcher)
	Expect(hec.KubernetesResource("PrometheusRule", "d8-monitoring", "extended-monitoring-exporter-health").Exists()).To(matcher)
}

var _ = Describe("Module :: extendedMonitoring :: helm template :: image availability ", func() {
	hec := SetupHelmConfig("")
	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
	})

	Context("With imageAvailability.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.imageAvailability.exporterEnabled", true)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			checkImageAvailabilityObjects(hec, true)
		})
	})
	Context("Without imageAvailability.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.imageAvailability.exporterEnabled", false)
			hec.HelmRender()
		})
		It("Should not deploy desired objects", func() {
			checkImageAvailabilityObjects(hec, false)
		})
	})
})
