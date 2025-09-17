/*
Copyright 2024 Flant JSC

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
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
enabledModules: ["vertical-pod-autoscaler"]
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.15.4
  prometheusScrapeInterval: 60
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

const module = `
oomKillsExporterEnabled: true
internal:
  clusterDNSImplementation: coredns
  vpa:
    kubeStateMetricsMaxCPU: 460m
    kubeStateMetricsMaxMemory: 870Mi
vpa: {}
`

var _ = Describe("Module :: monitoring-kubernetes :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("monitoringKubernetes", module)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		deploy := []string{"kube-state-metrics"}
		for _, i := range deploy {
			It(fmt.Sprintf("Deployment %s Exists", i), func() {
				test := f.KubernetesResource("Deployment", "d8-monitoring", i)
				Expect(test.Exists()).To(BeTrue())
			})
		}

		ds := []string{"node-exporter", "oom-kills-exporter"}
		for _, i := range ds {
			It(fmt.Sprintf("DaemonSet %s Exists", i), func() {
				test := f.KubernetesResource("DaemonSet", "d8-monitoring", i)
				Expect(test.Exists()).To(BeTrue())
			})
		}

		It("DaemonSet oom-kills-exporter check env", func() {
			test := f.KubernetesResource("DaemonSet", "d8-monitoring", "oom-kills-exporter")
			Expect(test.Field("spec.template.spec.containers.0.env").String()).To(MatchJSON(`
				[{ "name": "PROMETHEUS_SCRAPE_INTERVAL", "value": "60" }]
			`))
		})
	})
})
