/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
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
  "global": {
    "project": "test-me"
  },
  "flantIntegration": {
    "internal": {"madison": {"backends":["1.2.3.4"]}},
    "madisonAuthKey": "abc",
    "licenseKey": "abc"
  }
}`

		initConfigValuesString = `{}`
	)
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Project is active", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("values must be present", func() {
			Skip("Do not run madison backend test on CI, mock it first")

			Expect(f.ValuesGet("flantIntegration.internal.madison.backends").String()).
				To(MatchUnorderedJSON(`["54.38.235.70","54.38.235.72","54.38.235.73"]`))
		})
	})

	Context("No setup key", func() {
		BeforeEach(func() {
			f.ValuesDelete("flantIntegration.internal.licenseKey")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("values must be absent", func() {
			Expect(f.ValuesGet("flantIntegration.internal.madison.backends").Exists()).To(BeFalse())
		})
	})
})
