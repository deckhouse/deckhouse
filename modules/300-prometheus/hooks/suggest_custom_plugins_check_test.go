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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: suggest custom plugins check ::", func() {
	f := HookExecutionConfigInit(`{"prometheus":{"internal":{"grafana":{}}}}`, ``)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("After adding custom plugins to ModuleConfig", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("prometheus.grafana.customPlugins", []byte(`
---
- agenty-flowcharting-panel
- vertamedia-clickhouse-datasource
`))
				f.RunHook()
			})

			It("Should start exposing metrics about custom plugins", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(2))
				Expect(m[0].Name).To(Equal("d8_grafana_custom_plugins"))
				Expect(m[0].Labels).To(Equal(map[string]string{
					"plugin": "agenty_flowcharting_panel",
				}))
				Expect(m[1].Name).To(Equal("d8_grafana_custom_plugins"))
				Expect(m[1].Labels).To(Equal(map[string]string{
					"plugin": "vertamedia_clickhouse_datasource",
				}))
			})

			Context("And after deleting custom plugins from ModuleConfig", func() {
				BeforeEach(func() {
					f.ValuesDelete("prometheus.grafana.customPlugins")
					f.RunHook()
				})

				It("Should stop exposing metrics about custom plugins", func() {
					Expect(f).To(ExecuteSuccessfully())
					m := f.MetricsCollector.CollectedMetrics()
					Expect(m).To(HaveLen(0))
				})
			})
		})
	})
})
