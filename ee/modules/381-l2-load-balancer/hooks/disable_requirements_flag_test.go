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

var _ = Describe("l2LoadBalancer hooks :: disable requirements flag ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Module disable, requirements flag don't exist", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("The alert is set", func() {
			Expect(f).To(ExecuteSuccessfully())

			_, exists := requirements.GetValue(l2LoadBalancerModuleDeprecatedKey)
			Expect(exists).To(BeFalse())
		})

	})

})
