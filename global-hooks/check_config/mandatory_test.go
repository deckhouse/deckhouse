/*

User-stories:
1. There are mandatory fields `global.project` and `global.clusterName`. Hook must fail when the parameters aren't set.

*/

package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	initValuesString       = `{}`
	initConfigValuesString = `{"global": {}}`
)

var _ = Describe("Global hooks :: cluster_is_bootstraped ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Both `global.project` and `global.clusterName` aren't set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("`global.project` is set; `global.clusterName` isn't set", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("global.project", "ppp")
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("`global.project` isn't set; `global.clusterName` is set", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("global.clusterName", "ccc")
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("`global.project` isn't set; `global.clusterName` is set", func() {
		BeforeEach(func() {
			f.ConfigValuesSet("global.project", "ppp")
			f.ConfigValuesSet("global.clusterName", "ccc")
			f.BindingContexts.Set(OnStartupContext)
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

})
