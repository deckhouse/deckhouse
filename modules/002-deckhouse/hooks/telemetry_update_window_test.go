package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: deckhouse :: hooks :: update deckhouse image ::", func() {
	f := HookExecutionConfigInit(`{
        "global": {
          "modulesImages": {
			"registry": "my.registry.com/deckhouse"
		  }
        },
		"deckhouse": {
              "internal": {},
              "releaseChannel": "Stable",
			  "update": {
				"mode": "Auto",
				"windows": [{"from": "00:00", "to": "23:00"}]
			  }
			}
}`, `{}`)

	assertModeMetric := func(f *HookExecutionConfig, typeT string) {
		metrics := f.MetricsCollector.CollectedMetrics()

		Expect(metrics).ToNot(BeEmpty())

		expireIndex := -1
		for i, m := range metrics {
			if m.Action == "expire" && m.Group == "telemetry_deckhouse_update_window_approval_mode" {
				expireIndex = i
				break
			}
		}

		Expect(expireIndex >= 0).To(BeTrue())

		metricIndex := -1
		for i, m := range metrics {
			if m.Name == "telemetry_deckhouse_update_window_approval_mode" {
				Expect(m.Group).To(Equal("telemetry_deckhouse_update_window_approval_mode"))
				Expect(m.Value).To(Equal(pointer.Float64Ptr(1.0)))
				Expect(m.Labels).To(HaveKey("mode"))
				Expect(m.Labels["mode"]).To(Equal(typeT))
				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
		Expect(metricIndex > expireIndex).To(BeTrue())
	}

	Context("Approval mode is 'Manual'", func() {
		BeforeEach(func() {
			f.ValuesSet("deckhouse.update.mode", "Manual")
			f.ValuesSet("deckhouse.update.windows", []string{})

			f.BindingContexts.Set(f.GenerateScheduleContext("* */3 * * * *"))
			f.RunHook()
		})

		It("Sets mode metrics only", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertModeMetric(f, "Manual")
		})
	})
})
