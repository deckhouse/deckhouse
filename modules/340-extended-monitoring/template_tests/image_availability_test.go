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

func checkImageAvailabilityObjects(hec *Config, exist bool) {
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
			hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
			hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			checkImageAvailabilityObjects(hec, true)
		})
	})
	Context("Without imageAvailability.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.imageAvailability.exporterEnabled", false)
			hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
			hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
			hec.HelmRender()
		})
		It("Should not deploy desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			checkImageAvailabilityObjects(hec, false)
		})
	})

	Context("imageAvailability.ignoredImages", func() {
		Context("Empty", func() {
			BeforeEach(func() {
				hec.ValuesSet("extendedMonitoring.imageAvailability.exporterEnabled", true)
				hec.ValuesSet("extendedMonitoring.certificates.exporterEnabled", false)
				hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
				hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
				hec.HelmRender()
			})
			It("Should contain default ignored images", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				deploy := hec.KubernetesResource("Deployment", "d8-monitoring", "image-availability-exporter")
				ignoredImagesArg := deploy.Field("spec.template.spec.containers.0.args.1").String()

				Expect(ignoredImagesArg).To(Equal("--ignored-images=.*upmeter-nonexistent.*"))
			})
		})

		Context("Filled", func() {
			BeforeEach(func() {
				hec.ValuesSet("extendedMonitoring.imageAvailability.exporterEnabled", true)
				hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
				hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
				hec.ValuesSet("extendedMonitoring.imageAvailability.ignoredImages", []string{
					"a.b.com/zzz:9.7.1",
					"cr.k8s.io/xx-yy:4.3.1",
				})
				hec.HelmRender()
			})
			It("Should contain default and additional ignored images", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				deploy := hec.KubernetesResource("Deployment", "d8-monitoring", "image-availability-exporter")
				ignoredImagesArg := deploy.Field("spec.template.spec.containers.0.args.1").String()

				Expect(ignoredImagesArg).To(Equal("--ignored-images=.*upmeter-nonexistent.*~a.b.com/zzz:9.7.1~cr.k8s.io/xx-yy:4.3.1"))
			})
		})
	})
})
