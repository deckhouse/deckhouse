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
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

type datasource struct {
	Name     string `json:"name"`
	JSONData struct {
		TimeInterval string `json:"timeInterval"`
	} `json:"jsonData"`
}

type datasourcesConfig struct {
	Datasources       []datasource `json:"datasources"`
	DeleteDatasources []datasource `json:"deleteDatasources"`
}

var _ = Describe("Module :: prometheus :: helm template :: render data sources", func() {
	getGlobalValues := func(haEnabled bool) string {
		haStr := ""
		if haEnabled {
			haStr = "highAvailability: true"
		}

		return fmt.Sprintf(`
%s
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`, haStr)
	}

	getPrometheusValues := func(longtermRetentionDays int) string {
		return fmt.Sprintf(`
longtermRetentionDays: %d
scrapeInterval: 10s
auth: {}
vpa: {}
grafana: {}
https:
  mode: CustomCertificate
internal:
  vpa: {}
  prometheusMain: {}
  grafana: {}
  prometheusLongterm:
    retentionGigabytes: 1
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  alertmanagers: {}
  prometheusAPIClientTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  prometheusScraperIstioMTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  prometheusScraperTLS:
    certificate: CRTCRTCRT
    key: KEYKEYKEY
  auth: {}
`, longtermRetentionDays)

	}
	extractYamlDataFromData := func(createdSecret object_store.KubeObject, key string) *datasourcesConfig {
		var dataSources datasourcesConfig

		prometheusYamlEncoded := createdSecret.Field(fmt.Sprintf("data.%s", key)).String()
		prometheusYaml, err := base64.StdEncoding.DecodeString(prometheusYamlEncoded)
		Expect(err).ShouldNot(HaveOccurred())
		err = yaml.Unmarshal(prometheusYaml, &dataSources)
		Expect(err).ShouldNot(HaveOccurred())

		return &dataSources
	}

	assertDataSources := func(createdSecret object_store.KubeObject, countSources, countDeleted int) {
		dataSources := extractYamlDataFromData(createdSecret, "prometheus\\.yaml")
		Expect(dataSources.DeleteDatasources).To(HaveLen(countDeleted))
		Expect(dataSources.Datasources).To(HaveLen(countSources))
	}

	f := SetupHelmConfig(``)

	DescribeTable(
		"Grafana datasources secret was rendered correctly",
		func(haEnabled bool, longtermRetentionDays, countSources, countDeleted int) {
			f.ValuesSetFromYaml("global", getGlobalValues(haEnabled))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", getPrometheusValues(longtermRetentionDays))
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())

			createdSecret := f.KubernetesResource("Secret", "d8-monitoring", "grafana-datasources")
			Expect(createdSecret.Exists()).To(BeTrue())

			assertDataSources(createdSecret, countSources, countDeleted)

			additionalDataSourcesExists := createdSecret.Field("data.additional_datasources\\.yaml").Exists()
			Expect(additionalDataSourcesExists).To(BeFalse())
		},

		Entry("High availability and longterm enabled", true, 1, 5, 4),
		Entry("High availability enabled, longterm disabled", true, 0, 4, 5),
		Entry("High availability disabled, longterm enabled", false, 1, 3, 2),
		Entry("High availability and longterm disabled", false, 0, 2, 3),
	)

	Describe("Check Scrape Interval", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", getGlobalValues(true))
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("prometheus", getPrometheusValues(5))
			f.HelmRender()
		})

		It("Should has proper scrape interval", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			createdSecret := f.KubernetesResource("Secret", "d8-monitoring", "grafana-datasources")
			Expect(createdSecret.Exists()).To(BeTrue())

			dataSources := extractYamlDataFromData(createdSecret, "prometheus\\.yaml")
			for _, ds := range dataSources.Datasources {
				if strings.Contains(ds.Name, "longterm") {
					Expect(ds.JSONData.TimeInterval).To(Equal("5m"))
				} else {
					Expect(ds.JSONData.TimeInterval).To(Equal(f.ValuesGet("prometheus.scrapeInterval").String()))
				}
			}
		})
	})
})
