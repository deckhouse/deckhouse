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
	f.RegisterCRD("deckhouse.io", "v1", "CustomPrometheusRules", false)
	f.RegisterCRD("monitoring.coreos.com", "v1", "PrometheusRule", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
			f.RunHook()
		})

		Context("After adding CustomPrometheusRules", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: test
spec:
  groups:
  - name: gr1
    rules:
    - alert: Rule1
      expr: testit
  - name: gr2
    rules:
    - alert: Rule2
      expr: testit
`, 1))
				f.RunHook()
			})

			It("Should create PrometheusRule", func() {
				Expect(f).To(ExecuteSuccessfully())

				prometheusRule := f.KubernetesResource("PrometheusRule", "d8-monitoring", "d8-custom-test")
				Expect(prometheusRule.Exists()).To(BeTrue())
				Expect(prometheusRule.Field("spec.groups").String()).To(MatchYAML(`
- name: gr1
  rules:
  - alert: Rule1
    expr: testit
- name: gr2
  rules:
  - alert: Rule2
    expr: testit
`))
			})

			Context("And after updating CustomPrometheusRules", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: test
spec:
  groups:
  - name: update
    rules:
    - alert: updateRule1
      expr: testit
`, 2))
					f.RunHook()
				})

				It("Should update PrometheusRule", func() {
					Expect(f).To(ExecuteSuccessfully())

					prometheusRule := f.KubernetesResource("PrometheusRule", "d8-monitoring", "d8-custom-test")
					Expect(prometheusRule.Exists()).To(BeTrue())
					Expect(prometheusRule.Field("spec.groups").String()).To(MatchYAML(`
- name: update
  rules:
    - alert: updateRule1
      expr: testit
`))
				})

				Context("And after deleting CustomPrometheusRules", func() {
					BeforeEach(func() {
						f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
						f.RunHook()
					})

					It("Should delete PrometheusRule", func() {
						Expect(f).To(ExecuteSuccessfully())

						Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "d8-custom-test").Exists()).ToNot(BeTrue())
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
kind: CustomPrometheusRules
metadata:
  name: one
spec:
  groups:
  - name: gr1
    rules:
    - alert: Rule1
      expr: testit
---
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: two
spec:
  groups:
  - name: gr1
    rules:
    - alert: Rule1
      expr: testit
`, 2))
			f.RunHook()
		})

		It("Should synchronize the rules", func() {
			Expect(f).To(ExecuteSuccessfully())

			prometheusRule := f.KubernetesResource("PrometheusRule", "d8-monitoring", "d8-custom-one")
			Expect(prometheusRule.Exists()).To(BeTrue())
			Expect(prometheusRule.Field("spec.groups").String()).To(MatchYAML(`
- name: gr1
  rules:
  - alert: Rule1
    expr: testit
`))

			prometheusRuleNext := f.KubernetesResource("PrometheusRule", "d8-monitoring", "d8-custom-two")
			Expect(prometheusRuleNext.Exists()).To(BeTrue())
			Expect(prometheusRuleNext.Field("spec.groups").String()).To(MatchYAML(`
- name: gr1
  rules:
  - alert: Rule1
    expr: testit
`))
		})
	})

	Context("Cluster with prometheus rule but without custom prometheus rules", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: d8-custom-one
  namespace: d8-monitroing
  labels:
    module: prometheus
    heritage: deckhouse
    app: prometheus
    prometheus: main
    component: rules
    origin: custom
`, 1))
			f.RunHook()
		})

		It("Should delete prometheus rule on synchronization", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "d8-custom-one").Exists()).To(BeFalse())
		})
	})
})
