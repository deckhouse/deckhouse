/*
Copyright 2021 Flant CJSC

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: node_lease_handler ::", func() {
	const (
		stateNodes = `
---
apiVersion: v1
kind: Node
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Both lease and node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateLeases+stateNodes, 2))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Lease was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateNodes, 1))
				f.RunHook()
			})

			It("Hook must not fail", func() {
				Expect(f).To(ExecuteSuccessfully())

				currentTime := time.Now().UTC()
				Expect(f.KubernetesGlobalResource("Node", "node0").Field("status.conditions.1.lastHeartbeatTime").Time()).Should(BeTemporally("~", currentTime, time.Minute))
				Expect(f.KubernetesGlobalResource("Node", "node0").Field("status.conditions.1.lastTransitionTime").Time()).Should(BeTemporally("~", currentTime, time.Minute))

				Expect(f.KubernetesGlobalResource("Node", "node0").Field("status.conditions.1.message").String()).To(Equal("Status NotReady was set by node_lease_handler hook of node-manager Deckhouse module during bashible reboot step (candi/bashible/common-steps/all/099_reboot.sh)"))
				Expect(f.KubernetesGlobalResource("Node", "node0").Field("status.conditions.1.reason").String()).To(Equal("KubeletReady"))
				Expect(f.KubernetesGlobalResource("Node", "node0").Field("status.conditions.1.status").String()).To(Equal("False"))
				Expect(f.KubernetesGlobalResource("Node", "node0").Field("status.conditions.1.type").String()).To(Equal("Ready"))
			})
		})

	})

	Context("Only lease cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateLeases, 1))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Lease was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
				f.RunHook()
			})

			It("Hook must not fail", func() {
				Expect(f).To(ExecuteSuccessfully())
			})
		})
	})
})
