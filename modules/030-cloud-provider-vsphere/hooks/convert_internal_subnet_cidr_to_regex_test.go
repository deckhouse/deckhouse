package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/hook-testing/library"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	valuesString = `
{
  "cloudProviderVsphere": {
	"internal": {}
  }
}
`
	configValuesString = `{}`
	bindingContext     = `
[
  {
    "binding": "kubernetes",
    "type": "Event",
    "watchEvent": "Added",
    "object": {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {
        "name": "pod-321d12",
        "namespace": "default"
      }
    }
  }
]
`
)

var _ = Describe("", func() {
	SetupHookExecutionConfig(valuesString, configValuesString, bindingContext)

	BeforeEach(func() {
		HookConfig.ValuesSet("cloudProviderVsphere.internalSubnet", "172.16.3.0/24")
	})

	Context("with invalid subnet", func() {
		BeforeEach(func() {
			By("Setting invalid subnet")
			invalidSubnet := "esafds2222"
			HookConfig.ValuesSet("cloudProviderVsphere.internalSubnet", invalidSubnet)
		})

		Hook("should fail with non-zero exit code", func(hookResult *HookExecutionResult) {
			Expect(hookResult).ToNot(ExecuteSuccessfully())
		})
	})

	Context("with valid subnet", func() {
		Hook("should success with zero exit code", func(hookResult *HookExecutionResult) {
			Expect(hookResult).To(ExecuteSuccessfully())
			Expect(hookResult).To(ValuesHasKey("cloudProviderVsphere.internal.internalSubnetRegex"))
			Expect(hookResult).To(ValuesKeyEquals("cloudProviderVsphere.internal.internalSubnetRegex", "172\\.16\\.3\\.(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])"))
		})
	})
})
