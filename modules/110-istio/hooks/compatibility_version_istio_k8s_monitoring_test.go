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

package hooks

import (
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: check_istio_k8s_version_compatibility ::", func() {
	initValues := `
istio:
  internal:
    istioToK8sCompatibilityMap:
      "1.16": ["1.22", "1.23", "1.24", "1.25"]
      "1.19": ["1.25", "1.26", "1.27", "1.28"]
`
	f := HookExecutionConfigInit(initValues, "")

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(0))

		})
	})

	Context("Unknown version of istio", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", []byte(`["1.12", "1.16"]`))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.25.4")

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully and generate metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  monitoringMetricsGroup,
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_telemetry_istio_version_incompatible_with_k8s_version",
				Group:  monitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"istio_version": "1.12",
					"k8s_version":   "1.25.4",
				},
			}))
		})
	})

	Context("istio version known, but incompatible with current k8s version", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", []byte(`["1.16","1.19"]`))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.28.4")

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully and generate metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  monitoringMetricsGroup,
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_telemetry_istio_version_incompatible_with_k8s_version",
				Group:  monitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"istio_version": "1.16",
					"k8s_version":   "1.28.4",
				},
			}))
		})
	})

	Context(" the istio version is known, and it is compatible with the current version of k8s", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", []byte(`["1.16","1.19"]`))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.25.4")

			f.RunHook()
		})

		It("Hook must execute successfully and generate metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  monitoringMetricsGroup,
				Action: "expire",
			}))
		})
	})
})
