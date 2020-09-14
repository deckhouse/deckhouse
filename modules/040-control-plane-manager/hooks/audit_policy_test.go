/*

User-stories:
1. There is Secret kube-system/audit-policy with audit-policy.yaml set in data, hook must store it to `controlPlaneManager.internal.auditPolicy`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: audit_policy ::", func() {
	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = `{"controlPlaneManager":{"apiserver": {"auditPolicyEnabled": "false"}}}`
		stateA                 = `
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: c3RhdGVB
`
		stateB = `
apiVersion: v1
kind: Secret
metadata:
  name: audit-policy
  namespace: kube-system
data:
  audit-policy.yaml: c3RhdGVC
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("controlPlaneManager.internal.auditPolicy must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").Exists()).To(BeFalse())
		})
	})

	Context("Cluster started with stateA Secret and disabled auditPolicy", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").Exists()).To(BeFalse())
		})
	})

	Context("Cluster started with stateA Secret and not set auditPolicyEnabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.ConfigValuesDelete("controlPlaneManager.apiserver.auditPolicyEnabled")
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").Exists()).To(BeFalse())
		})
	})

	Context("Cluster started with stateA Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", "true")
			f.RunHook()
		})

		It("controlPlaneManager.internal.auditPolicy must be stateA", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String()).To(Equal("c3RhdGVB"))
		})

		Context("Cluster changed to stateB", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.ValuesSet("controlPlaneManager.apiserver.auditPolicyEnabled", "true")
				f.RunHook()
			})

			It("controlPlaneManager.internal.auditPolicy must be stateB", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.auditPolicy").String()).To(Equal("c3RhdGVC"))
			})
		})
	})

})
