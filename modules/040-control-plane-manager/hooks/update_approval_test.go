package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controlPlaneManager :: hooks :: update_approval ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("update_approval :: all flow for one pod", func() {
		state := `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  annotations:
    control-plane-manger.deckhouse.io/waiting-for-approval: ""
  labels:
      node-role.kubernetes.io/master: ""
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
  annotations:
    control-plane-manger.deckhouse.io/waiting-for-approval: ""
  labels:
      node-role.kubernetes.io/master: ""
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Node
metadata:
  name: worker-3
  annotations:
    control-plane-manger.deckhouse.io/waiting-for-approval: ""
  labels:
      node-role.kubernetes.io/master: ""
status:
  conditions:
  - type: Ready
    status: 'True'

---
apiVersion: v1
kind: Pod
metadata:
  name: d8-control-plane-manager-1
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  nodeName: worker-1
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  name: d8-control-plane-manager-2
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  name: d8-control-plane-manager-3
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  nodeName: worker-3
status:
  conditions:
  - type: Ready
    status: 'True'
`
		nodeNames := []string{"worker-1", "worker-2", "worker-3"}

		It("Works as expected", func() {
			approvedNodeIndex := -1

			By("one of nodes must be approved", func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, 1))
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())

				approvedCount := 0
				waitingForApprovalCount := 0
				for i := 1; i <= len(nodeNames); i++ {
					if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/approved`).Exists() {
						approvedCount++
						approvedNodeIndex = i
					}
					if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/waiting-for-approval`).Exists() {
						waitingForApprovalCount++
					}
				}

				Expect(approvedNodeIndex).To(Not(Equal(-1)))
				Expect(approvedCount).To(Equal(1))
				Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 1))

				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/approved`).Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())

				for i := 1; i <= len(nodeNames); i++ {
					if i == approvedNodeIndex {
						fmt.Println("EEE hi")
						continue
					}
					Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/approved`).Exists()).To(BeFalse())
				}
			})

			By(fmt.Sprintf("approved annotation should be removed from worker-%d when the pod is ready", approvedNodeIndex), func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(f.ObjectStore.ToYaml(), 1))
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/approved`).Exists()).To(BeFalse())
				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())

				for i := 1; i <= len(nodeNames); i++ {
					if i == approvedNodeIndex {
						fmt.Println("EEE hi")
						continue
					}
					Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
					Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/approved`).Exists()).To(BeFalse())
				}
			})

			By("next node must be approved", func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(f.ObjectStore.ToYaml(), 1))
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				approvedCount := 0
				waitingForApprovalCount := 0
				for i := 1; i <= len(nodeNames); i++ {
					if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/approved`).Exists() {
						approvedCount++
					}
					if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manger\.deckhouse\.io/waiting-for-approval`).Exists() {
						waitingForApprovalCount++
					}
				}

				Expect(approvedCount).To(Equal(1))
				Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 2))
			})
		})
	})
})
