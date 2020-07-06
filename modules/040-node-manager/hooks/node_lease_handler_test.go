package hooks

import (
	"time"

	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: node-manager :: hooks :: node_lease_handler ::", func() {
	const (
		stateNodes = `
---
apiVersion: v1
kind: Nodes
metadata:
  name: node0
status:
  conditions:
  - type: qqq
  - type: Ready
`
		stateLeases = `
---
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: node0
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Both lease and node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLeases + stateNodes))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Lease was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNodes))
				f.RunHook()
			})

			It("Hook must not fail", func() {
				Expect(f).To(ExecuteSuccessfully())

				current_time := time.Now().UTC()
				Expect(f.KubernetesGlobalResource("Nodes", "node0").Field("status.conditions.1.lastHeartbeatTime").Time()).Should(BeTemporally("~", current_time, time.Minute))
				Expect(f.KubernetesGlobalResource("Nodes", "node0").Field("status.conditions.1.lastTransitionTime").Time()).Should(BeTemporally("~", current_time, time.Minute))

				Expect(f.KubernetesGlobalResource("Nodes", "node0").Field("status.conditions.1.message").String()).To(Equal("Status NotReady was set by node_lease_handler hook of node-manager Deckhouse module during bashible reboot step (candi/bashible/common-steps/all/099_reboot.sh)"))
				Expect(f.KubernetesGlobalResource("Nodes", "node0").Field("status.conditions.1.reason").String()).To(Equal("KubeletReady"))
				Expect(f.KubernetesGlobalResource("Nodes", "node0").Field("status.conditions.1.status").String()).To(Equal("False"))
				Expect(f.KubernetesGlobalResource("Nodes", "node0").Field("status.conditions.1.type").String()).To(Equal("Ready"))
			})
		})

	})

	Context("Only lease cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLeases))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Lease was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.RunHook()
			})

			It("Hook must not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})
		})
	})
})
