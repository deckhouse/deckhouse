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

var _ = Describe("ingress-nginx :: hooks :: deprecated ::", func() {
	a := HookExecutionConfigInit(`{"global":{"enabledModules":["nginx-ingress"]}}`, `{}`)

	Context("Cluster with deprecated module enabled", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Metric values should be 1", func() {
			Expect(a).To(ExecuteSuccessfully())
			metrics := a.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Name).To(Equal("d8_nginx_ingress_deprecated"))
			expected := 1.0
			Expect(metrics[0].Value).To(Equal(&expected))
		})
	})

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Cluster without deprecated module enabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Metric values should be 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Name).To(Equal("d8_nginx_ingress_deprecated"))
			expected := 0.0
			Expect(metrics[0].Value).To(Equal(&expected))
		})
	})
})
