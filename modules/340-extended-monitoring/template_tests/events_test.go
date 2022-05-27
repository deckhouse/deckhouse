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

func checkEventsObjects(hec *Config, exist bool) {
	matcher := BeFalse()
	if exist {
		matcher = BeTrue()
	}

	Expect(hec.KubernetesResource("Deployment", "d8-monitoring", "events-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-monitoring", "events-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-monitoring", "events-exporter").Exists()).To(matcher)
	Expect(hec.KubernetesResource("ServiceAccount", "d8-monitoring", "events-exporter").Exists()).To(matcher)
}

var _ = Describe("Module :: extendedMonitoring :: helm template :: events ", func() {
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

	Context("With events.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.events.exporterEnabled", true)
			hec.ValuesSet("extendedMonitoring.events.severityLevel", "OnlyWarnings")
			hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability", `{}`)
			hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			checkEventsObjects(hec, true)
		})
	})
	Context("Without events.exporterEnabled", func() {
		BeforeEach(func() {
			hec.ValuesSet("extendedMonitoring.events.exporterEnabled", false)
			hec.ValuesSet("extendedMonitoring.events.severityLevel", "OnlyWarnings")
			hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability", `{}`)
			hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
			hec.HelmRender()
		})
		It("Should not deploy desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			checkEventsObjects(hec, false)
		})
	})
})
