/*
Copyright 2026 Flant JSC

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
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// monitoring.yaml renders, through helm_lib, the dashboards of monitoring/grafana-dashboards while the
// prometheus module is enabled, and the alerting rules of monitoring/prometheus-rules into d8-system
// while the PrometheusRule kind is installed, which operator-prometheus-crd stands for.

// deckhouseDashboards are the dashboards of monitoring/grafana-dashboards by the folder they go to.
var deckhouseDashboards = map[string]struct{ file, folder string }{
	"d8-deckhouse-main-deckhouse":                     {"main/deckhouse.json", "Main"},
	"d8-deckhouse-kubernetes-cluster-webhook-handler": {"kubernetes-cluster/webhook-handler.json", "Kubernetes Cluster"},
}

var _ = Describe("Module :: deckhouse :: monitoring ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	render := func(enabledModules string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSetFromYaml("global.enabledModules", enabledModules)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	rulesFiles := func() []string {
		files, err := filepath.Glob("../monitoring/prometheus-rules/*.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(files).ToNot(BeEmpty())
		return files
	}

	It("renders every dashboard of the module into its folder while prometheus is enabled", func() {
		render(`["prometheus"]`)

		files, err := filepath.Glob("../monitoring/grafana-dashboards/*/*.json")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(files).To(HaveLen(len(deckhouseDashboards)), "every dashboard of the module should be listed here")

		for name, source := range deckhouseDashboards {
			dashboard := f.KubernetesGlobalResource("GrafanaDashboardDefinition", name)
			Expect(dashboard.Exists()).To(BeTrue(), "the %s dashboard should be rendered", name)
			Expect(dashboard.Field("spec.folder").String()).To(Equal(source.folder), name)

			definition, err := os.ReadFile(filepath.Join("../monitoring/grafana-dashboards", source.file))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(dashboard.Field("spec.definition").String()).To(MatchJSON(definition), name)
		}
	})

	It("renders no dashboards while prometheus is disabled", func() {
		render(`["operator-prometheus-crd"]`)

		for name := range deckhouseDashboards {
			Expect(f.KubernetesGlobalResource("GrafanaDashboardDefinition", name).Exists()).To(BeFalse(), name)
		}
	})

	It("renders the alerting rules of every file into d8-system while the PrometheusRule kind is installed", func() {
		render(`["operator-prometheus-crd"]`)

		for _, file := range rulesFiles() {
			name := "deckhouse-" + strings.TrimSuffix(filepath.Base(file), ".yaml")
			rule := f.KubernetesResource("PrometheusRule", "d8-system", name)
			Expect(rule.Exists()).To(BeTrue(), "the rules of %s should be rendered as %s", file, name)
			Expect(rule.Field("spec.groups").Array()).ToNot(BeEmpty(), name)
		}
	})

	It("renders no alerting rules while the PrometheusRule kind is not installed", func() {
		render(`["prometheus", "operator-prometheus"]`)

		for _, file := range rulesFiles() {
			name := "deckhouse-" + strings.TrimSuffix(filepath.Base(file), ".yaml")
			Expect(f.KubernetesResource("PrometheusRule", "d8-system", name).Exists()).To(BeFalse(), name)
		}
	})
})
