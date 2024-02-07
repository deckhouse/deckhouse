/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: update_approval ::", func() {
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

	nodeNames := []string{"worker-1", "worker-2", "worker-3"}

	Context("update_approval :: all flow for one pod", func() {

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(initialState, 1))
			f.RunHook()
		})

		It("Works as expected", func() {
			approvedNodeIndex := -1
			Expect(f).To(ExecuteSuccessfully())

			approvedCount := 0
			waitingForApprovalCount := 0
			for i := 1; i <= len(nodeNames); i++ {
				if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/approved`).Exists() {
					approvedCount++
					approvedNodeIndex = i
				}
				if f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/waiting-for-approval`).Exists() {
					waitingForApprovalCount++
				}
			}

			Expect(approvedNodeIndex).To(Not(Equal(-1)))
			Expect(approvedCount).To(Equal(1))
			Expect(waitingForApprovalCount).To(Equal(len(nodeNames) - 1))

			Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/approved`).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", approvedNodeIndex)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())

			for i := 1; i <= len(nodeNames); i++ {
				if i == approvedNodeIndex {
					continue
				}
				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			}
		})
	})

	Context("worker-1 Node approved", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(approvedState, 1))
			f.RunHook()
		})
		It("approved annotation should be removed from worker-1 when the pod is ready", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesGlobalResource("Node", "worker-1").Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Node", "worker-1").Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeFalse())

			for i := 2; i <= len(nodeNames); i++ {
				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/waiting-for-approval`).Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("Node", fmt.Sprintf("worker-%d", i)).Field(`metadata.annotations.control-plane-manager\.deckhouse\.io/approved`).Exists()).To(BeFalse())
			}
		})
	})
})

var (
	stateTmpl = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
  annotations:
    %s
  labels:
      node-role.kubernetes.io/control-plane: ""
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
    control-plane-manager.deckhouse.io/waiting-for-approval: ""
  labels:
      node-role.kubernetes.io/control-plane: ""
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
    control-plane-manager.deckhouse.io/waiting-for-approval: ""
  labels:
      node-role.kubernetes.io/control-plane: ""
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
  - type: %s
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

	initialState = fmt.Sprintf(stateTmpl,
		"control-plane-manager.deckhouse.io/waiting-for-approval: \"\"",
		"NotReady",
	)

	approvedState = fmt.Sprintf(stateTmpl,
		"control-plane-manager.deckhouse.io/approved: \"\"",
		"Ready",
	)
)
