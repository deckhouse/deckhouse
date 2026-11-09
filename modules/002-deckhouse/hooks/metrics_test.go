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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: deckhouse :: hooks :: metrics ", func() {
	f := HookExecutionConfigInit(`{
        "global": {
          "modulesImages": {
			"registry": {
				"base": "my.registry.com/deckhouse"
			}
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

		metricIndex := -1
		for i, m := range metrics {
			if m.Name == "d8_telemetry_update_window_approval_mode" {
				Expect(m.Value).To(Equal(ptr.To(1.0)))
				Expect(m.Labels).To(HaveKey("mode"))
				Expect(m.Labels["mode"]).To(Equal(typeT))
				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
	}

	assertWindowMetric := func(f *HookExecutionConfig, from, to, days string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).ToNot(BeEmpty())

		metricIndex := -1
		dayFound := false
		for i, m := range metrics {
			if m.Name == "d8_telemetry_update_window" {
				if days != "" {
					if days != m.Labels["days"] {
						continue
					}

					dayFound = true
				}

				if from != m.Labels["from"] {
					continue
				}

				if to != m.Labels["to"] {
					continue
				}

				metricIndex = i
				break
			}
		}

		if days != "" {
			Expect(dayFound).To(BeTrue())
		}
		Expect(metricIndex >= 0).To(BeTrue())

		metric := metrics[metricIndex]
		Expect(metric.Value).To(Equal(ptr.To(1.0)))
		Expect(metric.Labels).To(HaveKey("from"))
		Expect(metric.Labels).To(HaveKey("to"))
	}

	assertOnlyModeAndReleaseChannelMetric := func(f *HookExecutionConfig) {
		metrics := f.MetricsCollector.CollectedMetrics()
		Expect(metrics).ToNot(BeEmpty())

		notExpireMetrics := 0
		for _, m := range metrics {
			if m.Action != operation.ActionExpireMetrics {
				notExpireMetrics++
			}
		}

		// + 1 - release channel
		Expect(notExpireMetrics).To(Equal(2))
	}

	registryModeMetrics := func(f *HookExecutionConfig) []operation.MetricOperation {
		var ops []operation.MetricOperation
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Group == "D8DeckhouseRegistryModeUnsupported" {
				ops = append(ops, m)
			}
		}
		return ops
	}

	registrySections := map[string]string{
		"Proxy":     `{"mode": "Proxy", "proxy": {"imagesRepo": "registry.deckhouse.io/test"}}`,
		"Local":     `{"mode": "Local"}`,
		"Unmanaged": `{"mode": "Unmanaged"}`,
		"Direct":    `{"mode": "Direct", "direct": {"imagesRepo": "registry.deckhouse.io/test"}}`,
	}

	for _, mode := range []string{"Proxy", "Local"} {
		Context("Registry mode is '"+mode+"'", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("deckhouse.registry", []byte(registrySections[mode]))

				f.RunHook()
			})

			It("Expires the group and sets the unsupported mode metric", func() {
				Expect(f).To(ExecuteSuccessfully())

				ops := registryModeMetrics(f)
				Expect(ops).To(HaveLen(2))
				Expect(ops[0].Action).To(Equal(operation.ActionExpireMetrics))
				Expect(ops[1].Action).To(Equal(operation.ActionGaugeSet))
				Expect(ops[1].Name).To(Equal("d8_deckhouse_registry_mode_unsupported"))
				Expect(ops[1].Value).To(Equal(ptr.To(1.0)))
				Expect(ops[1].Labels).To(Equal(map[string]string{"mode": mode}))
			})
		})
	}

	for _, mode := range []string{"Unmanaged", "Direct"} {
		Context("Registry mode is '"+mode+"'", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("deckhouse.registry", []byte(registrySections[mode]))

				f.RunHook()
			})

			It("Only expires the unsupported mode metric", func() {
				Expect(f).To(ExecuteSuccessfully())

				ops := registryModeMetrics(f)
				Expect(ops).To(HaveLen(1))
				Expect(ops[0].Action).To(Equal(operation.ActionExpireMetrics))
			})
		})
	}

	Context("Registry section is not set", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Only expires the unsupported mode metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			ops := registryModeMetrics(f)
			Expect(ops).To(HaveLen(1))
			Expect(ops[0].Action).To(Equal(operation.ActionExpireMetrics))
		})
	})

	Context("Approval mode is 'Manual'", func() {
		Context("Update window is not set", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.update.mode", "Manual")
				f.ValuesSet("deckhouse.update.windows", []string{})

				f.RunHook()
			})

			It("Executes hook successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Sets mode metric Manual", func() {
				assertModeMetric(f, "Manual")
			})

			It("Sets only one mode metric", func() {
				assertOnlyModeAndReleaseChannelMetric(f)
			})
		})

		Context("Update window is set", func() {
			BeforeEach(func() {
				f.ValuesSet("deckhouse.update.mode", "Manual")
				f.ValuesSetFromYaml("deckhouse.update.windows", []byte(`[{"from": "00:00", "to": "23:00"}]`))

				f.RunHook()
			})

			It("Executes hook successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Sets mode metric Manual", func() {
				assertModeMetric(f, "Manual")
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

			It("Executes hook successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Sets mode metric Auto", func() {
				assertModeMetric(f, "Auto")
			})

			It("Sets only one mode metric", func() {
				assertOnlyModeAndReleaseChannelMetric(f)
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

				Expect(f).To(ExecuteSuccessfully())
			})

			It("Executes hook successfully", func() {
				Expect(f).To(ExecuteSuccessfully())
			})

			It("Sets mode metric Auto", func() {
				assertModeMetric(f, "Auto")
			})

			It("Sets windows metric", func() {
				assertWindowMetric(f, "00:00", "23:00", "")
				assertWindowMetric(f, "01:00", "02:00", "Mon Fri")
			})
		})
	})
})
