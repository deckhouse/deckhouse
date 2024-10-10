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

var _ = Describe("Module :: extendedMonitoring :: helm template :: scrape timeout ", func() {
	hec := SetupHelmConfig("")
	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("extendedMonitoring.events.exporterEnabled", true)
		hec.ValuesSet("extendedMonitoring.events.severityLevel", "OnlyWarnings")
		hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability", `{}`)
		hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
	})

	Context("With lower scrape", func() {
		BeforeEach(func() {
			hec.ValuesSet("global.discovery.prometheusScrapeInterval", 10)
			hec.HelmRender()
		})
		It("Should be equal to scrape interval", func() {
			podMonitor := hec.KubernetesResource("PodMonitor", "d8-monitoring", "extended-monitoring-exporter")
			Expect(podMonitor.Exists()).To(BeTrue())

			scrapeTimeout := podMonitor.Field("spec.podMetricsEndpoints.0.scrapeTimeout").String()
			Expect(scrapeTimeout).Should(Equal("10s"))
		})
	})
	Context("With default scrape", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})
		It("Should be equal to timeout", func() {
			podMonitor := hec.KubernetesResource("PodMonitor", "d8-monitoring", "extended-monitoring-exporter")
			Expect(podMonitor.Exists()).To(BeTrue())

			scrapeTimeout := podMonitor.Field("spec.podMetricsEndpoints.0.scrapeTimeout").String()
			Expect(scrapeTimeout).Should(Equal("25s"))
		})
	})
})
