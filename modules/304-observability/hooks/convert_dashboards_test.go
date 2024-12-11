/*
Copyright 2021 Flant JSC

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

var _ = Describe("Prometheus hooks :: custom rules ::", func() {
	f := HookExecutionConfigInit(``, ``)
	f.RegisterCRD("observability.deckhouse.io", "v1alpha1", "ClusterObservabilityDashboard", false)
	f.RegisterCRD("deckhouse.io", "v1", "GrafanaDashboardDefinition", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
			f.RunHook()
		})

		Context("After adding GrafanaDashboardDefinition", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: test
spec:
  definition: "{}"
`, 1))
				f.RunHook()
			})

			It("Should create ClusterObservabilityDashboard", func() {
				Expect(f).To(ExecuteSuccessfully())

				dashboard := f.KubernetesResource("ClusterObservabilityDashboard", "", "test")
				Expect(dashboard.Exists()).To(BeTrue())
				Expect(dashboard.Field("spec.definition").String()).To(MatchJSON(`{}`))
			})

			Context("And after updating GrafanaDashboardDefinition", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: test
spec:
  definition: "{\"uid\": \"DEADBEEF\"}"
`, 2))
					f.RunHook()
				})

				It("Should update ClusterObservabilityDashboard", func() {
					Expect(f).To(ExecuteSuccessfully())

					dashboard := f.KubernetesResource("ClusterObservabilityDashboard", "", "test")
					Expect(dashboard.Exists()).To(BeTrue())
					Expect(dashboard.Field("spec.definition").String()).To(MatchJSON(`{"uid": "DEADBEEF"}`))
				})

				Context("And after deleting GrafanaDashboardDefinition", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
						f.RunHook()
					})

					It("Should delete ClusterObservabilityDashboard", func() {
						Expect(f).To(ExecuteSuccessfully())

						Expect(f.KubernetesResource("ClusterObservabilityDashboard", "", "test").Exists()).ToNot(BeTrue())
					})
				})
			})
		})
	})

	Context("Cluster with rules", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: one
spec:
  definition: |
    {
	  "uid": "foo"
	}
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: two
spec:
  definition: |
	{
	  "uid": "bar"
	}
`, 2))
			f.RunHook()
		})

		It("Should synchronize the rules", func() {
			Expect(f).To(ExecuteSuccessfully())

			dashboard := f.KubernetesResource("ClusterObservabilityDashboard", "", "one")
			Expect(dashboard.Exists()).To(BeTrue())
			Expect(dashboard.Field("spec.definition").String()).To(MatchJSON(`{"uid": "foo"}`))

			prometheusRuleNext := f.KubernetesResource("ClusterObservabilityDashboard", "", "two")
			Expect(prometheusRuleNext.Exists()).To(BeTrue())
			Expect(prometheusRuleNext.Field("spec.definition").String()).To(MatchYAML(`{"uid": "bar"}`))
		})
	})

	Context("Cluster with GrafanaDashboardDefinition but without ClusterObservabilityDashboard", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: test
`, 1))
			f.RunHook()
		})

		It("Should delete ClusterObservabilityDashboard on synchronization", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ClusterObservabilityDashboard", "", "test").Exists()).To(BeFalse())
		})
	})
})
