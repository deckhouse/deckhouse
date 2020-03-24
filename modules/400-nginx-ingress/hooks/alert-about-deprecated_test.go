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

var _ = Describe("Modules :: nginx-ingress :: hooks :: metrics ::", func() {
	const (
		initValuesString       = `{"nginxIngress":{"internal": {}}, "global": {"discovery":{"clusterType": "AWS"}}}`
		initConfigValuesString = `{"nginxIngress":{"inlet": "LoadBalancer", "additionalControllers":[{"name":"df", "inlet": "Direct"},{"name":"np", "inlet": "NodePort"},{"name":"lb", "inlet": "LoadBalancer"},{"name": "lb2"}]}}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.RunSchedule("*/10 * * * *"))
			f.RunHook()
		})

		It("must be executed successfully; countDepricatedInlets must be 4", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nginxIngress.internal.countDepricatedInlets").String()).To(Equal("4"))
		})
	})

})
