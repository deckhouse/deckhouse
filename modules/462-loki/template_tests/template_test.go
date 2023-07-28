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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	globalValues = `
enabledModules: ["vertical-pod-autoscaler-crd", "loki"]
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`
	lokiValues = `
internal:
  activated: true
  effectiveStorageClass: testStorageClass
resourcesManagement:
  mode: VPA
  vpa:
    mode: Auto
    cpu:
      min: "50m"
      max: "2"
    memory:
      min: "256Mi"
      max: "2Gi"
`
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: loki :: helm template ::", func() {
	f := SetupHelmConfig(``)
	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSetFromYaml("loki", lokiValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("Everything must render properly for empty cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})
	})

	Context("Check ClusterLoggingConfig for system logs", func() {

		It("Render chart with ClusterLoggingConfig", func() {
			f.ValuesSet("loki.storeSystemLogs", true)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			obj := f.KubernetesGlobalResource("ClusterLoggingConfig", "d8-namespaces-to-loki")
			Expect(obj.Exists()).To(BeTrue())
		})

		It("Render chart without ClusterLoggingConfig", func() {
			f.ValuesSet("loki.storeSystemLogs", false)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			obj := f.KubernetesGlobalResource("ClusterLoggingConfig", "d8-namespaces-to-loki")
			Expect(obj.Exists()).To(BeFalse())
		})
	})

	Context("Check GrafanaAdditionalDatasource render", func() {

		It("Render chart with grafana datasource if prometheus-crd enabled", func() {
			f.ValuesSet("global.enabledModules", []string{"loki", "prometheus-crd"})
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			obj := f.KubernetesGlobalResource("GrafanaAdditionalDatasource", "d8-loki")
			Expect(obj.Exists()).To(BeTrue())
		})

		It("Render chart without grafana datasource if prometheus-crd disabled", func() {
			f.ValuesSet("global.enabledModules", []string{"loki"})
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			obj := f.KubernetesGlobalResource("GrafanaAdditionalDatasource", "d8-loki")
			Expect(obj.Exists()).To(BeFalse())
		})
	})
})
