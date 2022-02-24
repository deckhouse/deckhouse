/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package madison

import (
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: madison backends discovery ::", func() {
	const (
		initValuesString = `
{
  "global": {},
  "flantIntegration": {
    "internal": {"madison": {"backends":["1.2.3.4"]}, "licenseKey": "abc"},
    "madisonAuthKey": "abc"
  }
}`

		initConfigValuesString          = `{}`
		initConfigValuesWithProxyString = `
{
  "global": {
    "modules": {
      "proxy": {
        "httpProxy": "1.2.3.4:8080"
      }
    }
  }
}`
	)
	a := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Project is active", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.GenerateScheduleContext("*/10 * * * *"))
			a.RunHook()
		})

		It("values must be present", func() {
			Skip("Do not run madison backend test on CI, mock it first")

			Expect(a.ValuesGet("flantIntegration.internal.madison.backends").String()).
				To(MatchOrderedJSON(`["54.38.235.70:443","54.38.235.72:443","54.38.235.73:443"]`))
		})
	})

	Context("No setup key", func() {
		BeforeEach(func() {
			a.ValuesDelete("flantIntegration.internal.licenseKey")
			a.BindingContexts.Set(a.GenerateScheduleContext("*/10 * * * *"))
			a.RunHook()
		})

		It("values must be absent", func() {
			Expect(a.ValuesGet("flantIntegration.internal.madison.backends").Exists()).To(BeFalse())
		})
	})

	b := HookExecutionConfigInit(initValuesString, initConfigValuesWithProxyString)

	Context("Project is active, httpProxy is set", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateScheduleContext("*/10 * * * *"))
			b.RunHook()
		})

		It("values must be present", func() {
			Expect(b.ValuesGet("flantIntegration.internal.madison.backends").String()).
				To(MatchOrderedJSON(`["1.2.3.4:8080"]`))
		})
	})

})
