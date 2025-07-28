/*
Copyright 2025 Flant JSC

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

var moduleConfigWithEdition = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    license:
      edition: CE
`
var moduleConfigWithoutEdition = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
`

var _ = Describe("Modules :: deckhouse :: hooks :: check_deckhouse_edition ::", func() {
	f := HookExecutionConfigInit(`{
        "global": {
          "modulesImages": {
			"registry": {
			}
		  }
        }
}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("No moduleconfig and edition", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Executes correctly", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).ToNot(BeEmpty())
			for _, m := range metrics {
				if m.Name == "d8_edition_not_found" {
					Expect(*m.Value).To(Equal(1.0))
				}
			}
		})
	})

	Context("Edition in moduleconfig ", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(moduleConfigWithEdition)
			f.BindingContexts.Set(st)
			f.RunHook()
		})

		It("Metrics must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).ToNot(BeEmpty())
			for _, m := range metrics {
				if m.Name == "d8_edition_not_found" {
					Expect(*m.Value).To(Equal(0.0))
				}
			}
		})
	})

	Context("Edition in global value ", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(moduleConfigWithoutEdition)
			f.BindingContexts.Set(st)
			f.ValuesSet("global.deckhouseEdition", "CE")
			f.RunHook()
			f.ValuesDelete("global.deckhouseEdition")
		})

		It("Metrics must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).ToNot(BeEmpty())
			for _, m := range metrics {
				if m.Name == "d8_edition_not_found" {
					Expect(*m.Value).To(Equal(0.0))
				}
			}
		})
	})

	Context("Edition in registry path", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(moduleConfigWithoutEdition)
			f.BindingContexts.Set(st)
			f.ValuesSet("global.modulesImages.registry.address", "registry.deckhouse.io")
			f.ValuesSet("global.modulesImages.registry.path", "/deckhouse/ce")
			f.RunHook()
			f.ValuesDelete("global.modulesImages.registry.address")
			f.ValuesDelete("global.modulesImages.registry.path")
		})

		It("Metrics must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).ToNot(BeEmpty())
			for _, m := range metrics {
				if m.Name == "d8_edition_not_found" {
					Expect(*m.Value).To(Equal(0.0))
				}
			}
		})
	})
})
