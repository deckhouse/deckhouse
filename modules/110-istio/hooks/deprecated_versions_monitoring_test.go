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

var _ = Describe("Istio hooks :: versions_monitoring ::", func() {
	f := HookExecutionConfigInit(``, ``)
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

	Context("There are no deprecated versions", func() {
		var noDeprecatedVersions = `
globalVersion: 1.1.1
additionalVersions:
- 1.2.0
- 1.3.0
`
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(noDeprecatedVersions))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  versionsMonitoringMetricsGroup,
				Action: "expire",
			}))
		})
	})

	Context("There are no deprecated version installed", func() {
		var noDeprecatedVersions = `
globalVersion: 1.1.1
additionalVersions:
- 1.2.0
- 1.3.0
internal:
   deprecatedVersions:
   - version: 1.1.9
     alertSeverity: 4
`
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(noDeprecatedVersions))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  versionsMonitoringMetricsGroup,
				Action: "expire",
			}))
		})
	})

	Context("There is one deprecated version installed", func() {
		var noDeprecatedVersions = `
globalVersion: 1.1.1
additionalVersions:
- 1.2.0
- 1.3.0
internal:
   deprecatedVersions:
   - version: 1.1.1
     alertSeverity: 4
`
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(noDeprecatedVersions))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  versionsMonitoringMetricsGroup,
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_deprecated_version_installed",
				Group:  versionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"version":        "1.1.1",
					"alert_severity": "4",
				},
			}))
		})
	})

	Context("There are several deprecated version installed", func() {
		var noDeprecatedVersions = `
globalVersion: 1.1.1
additionalVersions:
- 1.2.0
- 1.3.0
internal:
   deprecatedVersions:
   - version: 1.2.0
     alertSeverity: 8
   - version: 1.3.0
     alertSeverity: 9

`
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio", []byte(noDeprecatedVersions))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(3))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  versionsMonitoringMetricsGroup,
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_deprecated_version_installed",
				Group:  versionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"version":        "1.2.0",
					"alert_severity": "8",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_istio_deprecated_version_installed",
				Group:  versionsMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"version":        "1.3.0",
					"alert_severity": "9",
				},
			}))

		})
	})
})
