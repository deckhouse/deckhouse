// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global :: hooks :: telemetry :: enable module", func() {
	f := HookExecutionConfigInit(`{
        "global": {
          "enabledModules": ["control-plane-manager"],
          "modulesImages": {
			"registry": "my.registry.com/deckhouse"
		  }
        }
}`, `{}`)

	assertModuleMetric := func(f *HookExecutionConfig, module string, val float64) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(len(metrics)).To(Equal(2))
		Expect(metrics[0].Action).To(Equal("expire"))
		Expect(metrics[0].Group).To(Equal("d8_telemetry_telemetry_modules_enable"))

		found := false
		for i := 1; i < len(metrics); i++ {
			if metrics[i].Name == fmt.Sprintf("d8_telemetry_%s_module_enabled", module) {
				found = true
				Expect(*metrics[i].Value).To(Equal(val))
				break
			}
		}

		Expect(found).To(BeTrue())
	}

	Context("Istio module is enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.enabledModules", []string{"a", "b", "istio", "d"})

			f.RunHook()
		})

		It("Sets enabled metric as 1.0 value", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertModuleMetric(f, "istio", 1.0)
		})

	})

	Context("Istio module is disabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.enabledModules", []string{"a", "b", "d"})

			f.RunHook()
		})

		It("Sets enabled metric as 0.0 value", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertModuleMetric(f, "istio", 0.0)
		})

	})
})
