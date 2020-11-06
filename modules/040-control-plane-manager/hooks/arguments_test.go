package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: arguments ::", func() {

	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = `{"controlPlaneManager":{"apiserver": {"auditPolicyEnabled": "false"}}}}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("controlPlaneManager.internal.arguments must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.arguments").Exists()).To(BeFalse())
		})

		Context("nodeMonitorGracePeriodSeconds is set to 15 seconds", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.nodeMonitorGracePeriodSeconds", "15")
				f.RunHook()
			})

			It("arguments must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.arguments").String()).To(MatchJSON(`{"nodeMonitorPeriod": 2, "nodeMonitorGracePeriod": 15}`))
			})
		})

		Context("failedNodePodEvictionTimeoutSeconds is set to 15 seconds", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.failedNodePodEvictionTimeoutSeconds", "15")
				f.RunHook()
			})

			It("arguments must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.arguments").String()).To(MatchJSON(`{"podEvictionTimeout": 15, "defaultUnreachableTolerationSeconds": 15}`))
			})
		})

		Context("nodeMonitorGracePeriodSeconds and failedNodePodEvictionTimeoutSeconds both are set to 15 seconds", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.nodeMonitorGracePeriodSeconds", "15")
				f.ValuesSet("controlPlaneManager.failedNodePodEvictionTimeoutSeconds", "15")
				f.RunHook()
			})

			It("arguments must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.arguments").String()).To(MatchJSON(`{"nodeMonitorPeriod": 2, "nodeMonitorGracePeriod": 15, "podEvictionTimeout": 15, "defaultUnreachableTolerationSeconds": 15}`))
			})
		})

	})

})
