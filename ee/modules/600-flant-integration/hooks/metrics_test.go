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
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("helm :: hooks :: metrics ::", func() {
	f := HookExecutionConfigInit("", "")

	Context("Initial empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		Context("madison integration is disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("flantIntegration.madisonAuthKey", false)
				f.RunGoHook()
			})
			It("d8_flant_integration_misconfiguration_detected must be 0", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				metric := metrics[0]
				Expect(metric.Name).To(Equal("d8_flant_integration_misconfiguration_detected"))
				Expect(*metric.Value).To(Equal(float64(0)))
			})
		})
		Context("metrics shipment is disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("flantIntegration.madisonAuthKey", "secret")
				f.ValuesSet("flantIntegration.metrics", false)
				f.RunGoHook()
			})
			It("d8_flant_integration_misconfiguration_detected must be 1", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				metric := metrics[0]
				Expect(metric.Name).To(Equal("d8_flant_integration_misconfiguration_detected"))
				Expect(*metric.Value).To(Equal(float64(1)))
			})
		})
		Context("kubeall host is not set", func() {
			BeforeEach(func() {
				f.ValuesSet("flantIntegration.madisonAuthKey", "secret")
				f.ValuesSet("flantIntegration.metrics", "https://connect.deckhouse.io/v1/remote_write")
				f.RunGoHook()
			})
			It("d8_flant_integration_misconfiguration_detected must be 2", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				metric := metrics[0]
				Expect(metric.Name).To(Equal("d8_flant_integration_misconfiguration_detected"))
				Expect(*metric.Value).To(Equal(float64(2)))
			})
		})
		Context("configuration is ok", func() {
			BeforeEach(func() {
				f.ValuesSet("flantIntegration.madisonAuthKey", "secret")
				f.ValuesSet("flantIntegration.metrics", "https://connect.deckhouse.io/v1/remote_write")
				f.ValuesSet("flantIntegration.kubeall.host", "my.kube-master")
				f.RunGoHook()
			})
			It("d8_flant_integration_misconfiguration_detected must be 0", func() {
				Expect(f).To(ExecuteSuccessfully())
				metrics := f.MetricsCollector.CollectedMetrics()
				Expect(metrics).To(HaveLen(1))
				metric := metrics[0]
				Expect(metric.Name).To(Equal("d8_flant_integration_misconfiguration_detected"))
				Expect(*metric.Value).To(Equal(float64(0)))
			})
		})
	})
})
