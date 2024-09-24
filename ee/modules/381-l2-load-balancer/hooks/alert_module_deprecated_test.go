/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("l2LoadBalancer hooks :: set alert for deprecated module ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "L2LoadBalancer", false)

	Context("Module enable, l2LoadBalancer object exits in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("The alert is set", func() {
			Expect(f).To(ExecuteSuccessfully())

			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Name).To(Equal("d8_l2_load_balancer_module_enabled"))
			Expect(*metrics[0].Value).To(Equal(float64(1)))
			isModuleIsEnabled, exists := requirements.GetValue(l2LoadBalancerModuleDeprecatedKey)
			Expect(exists).To(BeTrue())
			Expect(isModuleIsEnabled).To(BeTrue())
		})

	})

})
