/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
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
				f.ValuesSet("flantIntegration.metrics.url", "https://connect.deckhouse.io/v1/remote_write")
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
				f.ValuesSet("flantIntegration.metrics.url", "https://connect.deckhouse.io/v1/remote_write")
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
