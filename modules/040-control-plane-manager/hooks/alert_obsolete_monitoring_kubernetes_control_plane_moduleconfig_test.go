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

var _ = Describe("Modules :: control-plane-manager :: hooks :: alert on obsolete monitoring-kubernetes-control-plane ModuleConfig", func() {
	const obsoleteMC = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes-control-plane
spec:
  enabled: true
`
	const otherMC = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  enabled: true
`

	Context("When the obsolete monitoring-kubernetes-control-plane ModuleConfig is absent", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(otherMC))
			f.RunHook()
		})

		It("Does not set the alert metric and keeps other ModuleConfigs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleConfig", "control-plane-manager").Exists()).To(BeTrue())

			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == obsoleteMonitoringKubernetesControlPlaneMetric {
					Expect(m.Value).To(BeNil())
				}
			}
		})
	})

	Context("When the obsolete monitoring-kubernetes-control-plane ModuleConfig exists", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(obsoleteMC + otherMC))
			f.RunHook()
		})

		It("Sets the alert metric and keeps the ModuleConfig", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("ModuleConfig", "monitoring-kubernetes-control-plane").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ModuleConfig", "control-plane-manager").Exists()).To(BeTrue())

			var found bool
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == obsoleteMonitoringKubernetesControlPlaneMetric {
					found = true
					Expect(*m.Value).To(Equal(1.0))
					Expect(m.Labels).To(HaveKeyWithValue("moduleconfig", "monitoring-kubernetes-control-plane"))
				}
			}
			Expect(found).To(BeTrue())
		})
	})
})
