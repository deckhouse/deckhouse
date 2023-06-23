/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"time"

	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus :: hooks :: set lookbackDelta ::", func() {
	const testScrapeInterval = time.Duration(30) * time.Second
	const testLookbackDelta = time.Duration(60) * time.Second

	Context(fmt.Sprintf("set lookbackDelta to minimum %s if scrapeInterval is shorter than %s", minLookbackDelta, minLookbackDelta), func() {
		f := HookExecutionConfigInit(`{"global": {}, "prometheus": {"internal":{"prometheusMain":{}}}}`, `{"prometheus":{"scrapeInterval": "1s"}}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It(fmt.Sprintf("should set lookbackDelta to %s minimum", minLookbackDelta), func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.lookbackDelta").String()).To(Equal(minLookbackDelta.String()))
		})
	})

	Context("set lookbackDelta value to 2x scrapeInterval", func() {
		f := HookExecutionConfigInit(`{"global": {}, "prometheus": {"internal":{"prometheusMain":{}}}}`, fmt.Sprintf(`{"prometheus":{"scrapeInterval": "%s"}}`, testScrapeInterval))

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set lookbackDelta value to 2x scrapeInterval", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.prometheusMain.lookbackDelta").String()).To(Equal(testLookbackDelta.String()))
		})
	})
})
