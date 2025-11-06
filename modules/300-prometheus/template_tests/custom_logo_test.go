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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: prometheus :: helm template :: grafana custom logo", func() {
	f := SetupHelmConfig(``)
	values := `
auth: {}
vpa: {}
grafana: {}
https:
  mode: CustomCertificate
internal:
  vpa: {}
  prometheusMain: {}
  grafana:
    enabled: true
  customCertificateData:
    tls.crt: |
      -----BEGIN CERTIFICATE-----
      TEST
      -----END CERTIFICATE-----
    tls.key: |
      -----BEGIN PRIVATE KEY-----
      TEST
      -----END PRIVATE KEY-----
  alertmanagers: {}
  prometheusAPIClientTLS: {}
  prometheusScraperIstioMTLS: {}
  prometheusScraperTLS: {}
  auth: {}
`

	Context("CustomLogo disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", values)
			f.HelmRender()
		})

		It("Deployment must not have volume and volumeMount", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			resource := f.KubernetesResource("Deployment", "d8-monitoring", "grafana-v10")
			Expect(resource.Exists()).To(BeTrue())

			var contains bool

			// Check volume exists
			volumes := resource.Field("spec.template.spec.volumes").Array()
			for _, volume := range volumes {
				if volume.Map()["name"].String() == "whitelabel-logo" {
					contains = true
					break
				}
			}
			Expect(contains).To(BeFalse())
			// end volume check

			// Check volume mount
			volumeMounts := resource.Field("spec.template.spec.containers.0.volumeMounts").Array()
			for _, vm := range volumeMounts {
				if vm.Map()["name"].String() == "whitelabel-logo" {
					contains = true
					break
				}
			}
			Expect(contains).To(BeFalse())
			// end volume mount check
		})
	})

	Context("CustomLogo enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("prometheus", values)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("prometheus.internal.grafana.customLogo.enabled", true)
			f.ValuesSet("prometheus.internal.grafana.customLogo.checksum", "abc")
			f.HelmRender()
		})

		It("Deployment must have volume and volumeMount", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			resource := f.KubernetesResource("Deployment", "d8-monitoring", "grafana-v10")
			Expect(resource.Exists()).To(BeTrue())

			// Check volume exists
			volumes := resource.Field("spec.template.spec.volumes").Array()
			var contains bool
			for _, volume := range volumes {
				if volume.Map()["name"].String() == "whitelabel-logo" {
					contains = true
					break
				}
			}
			Expect(contains).To(BeTrue())
			// end volume check

			// Check volume mount
			volumeMounts := resource.Field("spec.template.spec.containers.0.volumeMounts").Array()
			for _, vm := range volumeMounts {
				if vm.Map()["name"].String() == "whitelabel-logo" {
					contains = true
					break
				}
			}
			Expect(contains).To(BeTrue())
			// end volume mount check
		})
	})
})
