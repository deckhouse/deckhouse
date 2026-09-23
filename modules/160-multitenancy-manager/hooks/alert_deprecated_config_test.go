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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: alert_deprecated_config ::", func() {
	const withParameter = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: multitenancy-manager
spec:
  enabled: true
  settings:
    allowNamespacesWithoutProjects: false
`
	const withParameterTrue = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: multitenancy-manager
spec:
  enabled: true
  settings:
    allowNamespacesWithoutProjects: true
`
	const withoutParameter = `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: multitenancy-manager
spec:
  enabled: true
  settings:
    highAvailability: true
`
	f := HookExecutionConfigInit(`{"multitenancyManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	// collected returns the deprecated-parameter series the hook set, ignoring the group expire.
	collected := func() []map[string]string {
		var out []map[string]string
		for _, m := range f.MetricsCollector.CollectedMetrics() {
			if m.Name == deprecatedConfigMetric {
				out = append(out, m.Labels)
			}
		}
		return out
	}

	Context("the ModuleConfig sets the deprecated parameter to false", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(withParameter))
			f.RunHook()
		})
		It("exports the metric for it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(collected()).To(ConsistOf(map[string]string{
				"module":    "multitenancy-manager",
				"parameter": "allowNamespacesWithoutProjects",
			}))
		})
	})

	Context("the ModuleConfig sets the deprecated parameter to true", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(withParameterTrue))
			f.RunHook()
		})
		It("exports the metric as well: presence is what matters, neither value does anything", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(collected()).To(HaveLen(1))
		})
	})

	Context("the ModuleConfig does not set the deprecated parameter", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(withoutParameter))
			f.RunHook()
		})
		It("exports nothing and only expires the group", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(collected()).To(BeEmpty())
		})
	})

	Context("there is no ModuleConfig at all", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("exports nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(collected()).To(BeEmpty())
		})
	})
})
