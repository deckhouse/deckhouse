/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func checkCertExporterObjects(hec *Config, exist bool) {
	matcher := BeFalse()
	if exist {
		matcher = BeTrue()
	}

	Expect(hec.KubernetesResource("Deployment", "d8-monitoring", "cert-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "cert-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-monitoring", "cert-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("ServiceAccount", "d8-monitoring", "cert-exporter").Exists()).To(matcher)
}

var _ = Describe("Module :: extendedMonitoring :: helm template :: cert-exporter ", func() {
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

	Context("With certificates.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.certificates.exporterEnabled", true)
			hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability", `{}`)
			hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			checkCertExporterObjects(hec, true)
		})
	})
	Context("Without imageAvailability.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.certificates.exporterEnabled", false)
			hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability", `{}`)
			hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
			hec.HelmRender()
		})
		It("Should not deploy desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			checkCertExporterObjects(hec, false)
		})
	})
})
