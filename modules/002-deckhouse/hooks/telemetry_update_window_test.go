/*
Copyright 2022 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: telemetry :: update window", func() {
	f := HookExecutionConfigInit(`{
        "global": {
          "modulesImages": {
			"registry": "my.registry.com/deckhouse"
		  }
        },
		"deckhouse": {
              "internal": {},
              "releaseChannel": "Stable",
			  "update": {
				"mode": "Auto",
				"windows": [{"from": "00:00", "to": "23:00"}]
			  }
			}
}`, `{}`)

	assertModeMetric := func(f *HookExecutionConfig, typeT string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).ToNot(BeEmpty())

		expireIndex := -1
		for i, m := range metrics {
			if m.Action == "expire" && m.Group == "d8_telemetry_update_window_approval_mode" {
				expireIndex = i
				break
			}
		}

		Expect(expireIndex >= 0).To(BeTrue())

		metricIndex := -1
		for i, m := range metrics {
			if m.Name == "d8_telemetry_update_window_approval_mode" {
				Expect(m.Group).To(Equal("d8_telemetry_update_window_approval_mode"))
				Expect(m.Value).To(Equal(pointer.Float64Ptr(1.0)))
				Expect(m.Labels).To(HaveKey("mode"))
				Expect(m.Labels["mode"]).To(Equal(typeT))
				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
		Expect(metricIndex > expireIndex).To(BeTrue())
	}

	assertWindowMetric := func(f *HookExecutionConfig, from, to, day, human string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).ToNot(BeEmpty())

		expireIndex := -1
		for i, m := range metrics {
			if m.Action == "expire" && m.Group == "d8_telemetry_update_window" {
				expireIndex = i
				break
			}
		}

		Expect(expireIndex >= 0).To(BeTrue())

		metricIndex := -1
		for i, m := range metrics {
			if m.Name == "d8_telemetry_update_window" {
				Expect(m.Group).To(Equal("d8_telemetry_update_window"))
				Expect(m.Value).To(Equal(pointer.Float64Ptr(1.0)))
				Expect(m.Labels).To(HaveKey("from"))
				Expect(m.Labels).To(HaveKey("to"))
				Expect(m.Labels).To(HaveKey("human"))
				if day != "" {
					Expect(m.Labels).To(HaveKey("human"))
					if day != m.Labels["day"] {
						continue
					}
				}

				if from != m.Labels["from"] {
					continue
				}

				if to != m.Labels["to"] {
					continue
				}

				if human != m.Labels["human"] {
					continue
				}

				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
		Expect(metricIndex > expireIndex).To(BeTrue())
	}

	assertOnlyModeMetric := func(f *HookExecutionConfig) {
		metrics := f.MetricsCollector.CollectedMetrics()
		Expect(metrics).ToNot(BeEmpty())

		notExpireMetrics := 0
		for _, m := range metrics {
			if m.Action != "expire" {
				notExpireMetrics += 1
			}
		}

		Expect(notExpireMetrics).To(Equal(1))
	}

	Context("Approval mode is 'Manual'", func() {
		Context("Update window is not set", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.update.mode", "Manual")
				f.ValuesSet("deckhouse.update.windows", []string{})

				f.RunHook()
			})

			It("Sets mode metric Manual", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertModeMetric(f, "Manual")
			})

			It("Sets only one mode metric", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertOnlyModeMetric(f)
			})
		})

		Context("Update window is set", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.update.mode", "Manual")
				f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "00:00", "to": "23:00"}]`))

				f.RunHook()
			})

			It("Sets mode metric Manual", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertModeMetric(f, "Manual")
			})

			It("Sets only one mode metric", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertOnlyModeMetric(f)
			})
		})
	})

	Context("Approval mode is 'Auto'", func() {
		Context("Update window is not set", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.update.mode", "Auto")
				f.ValuesSet("deckhouse.update.windows", []string{})

				f.RunHook()
			})

			It("Sets mode metric Auto", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertModeMetric(f, "Auto")
			})

			It("Sets only one mode metric", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertOnlyModeMetric(f)
			})
		})

		Context("Update window is set", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.update.mode", "Auto")
				f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`
[
  {"from": "00:00", "to": "23:00"},
  {"from": "01:00", "to": "02:00", "days":["Mon", "Fri"]}
]`))

				f.RunHook()
			})

			It("Sets mode metric Auto", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertModeMetric(f, "Auto")
			})

			It("Sets windows metric", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertWindowMetric(f, "00:00", "23:00", "", "00:00 - 23:00")
				assertWindowMetric(f, "01:00", "02:00", "Mon", "Mon - 01:00 - 02:00")
				assertWindowMetric(f, "01:00", "02:00", "Fri", "Fri - 01:00 - 02:00")
			})
		})
	})
})
