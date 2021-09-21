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

var _ = Describe("Modules :: node-manager :: hooks :: chaos_monkey ::", func() {
	const (
		stateCloudNGSmall = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: too-small
spec:
  nodeType: CloudEphemeral
status:
  desired: 1
  ready: 1
`
		stateCloudNGLarge = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: largeng
spec:
  nodeType: CloudEphemeral
  chaos:
    mode: DrainAndDelete
    period: 5m
status:
  desired: 3
  ready: 3
`
		stateCloudNGLargeBroken = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: largeng
spec:
  nodeType: CloudEphemeral
  chaos:
    mode: DrainAndDelete
    period: 5m
status:
  desired: 3
  ready: 2
`

		stateHybridNGSmall = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: too-small
spec:
  nodeType: CloudPermanent
status:
  nodes: 1
  ready: 1
`
		stateHybridNGLarge = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: largeng
spec:
  nodeType: CloudPermanent
  chaos:
    mode: DrainAndDelete
    period: 5m
status:
  nodes: 3
  ready: 3
`
		stateHybridNGLargeBroken = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: largeng
spec:
  nodeType: CloudPermanent
  chaos:
    mode: DrainAndDelete
    period: 5m
status:
  nodes: 3
  ready: 2
`

		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: largeng
---
apiVersion: v1
kind: Node
metadata:
  name: node2
  labels:
    node.deckhouse.io/group: largeng
---
apiVersion: v1
kind: Node
metadata:
  name: node3
  labels:
    node.deckhouse.io/group: largeng
---
apiVersion: v1
kind: Node
metadata:
  name: smallnode1
  labels:
    node.deckhouse.io/group: too-small
`
		stateMachines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: node1
  namespace: d8-cloud-instance-manager
  labels:
    node: node1
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: node2
  namespace: d8-cloud-instance-manager
  labels:
    node: node2
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: node3
  namespace: d8-cloud-instance-manager
  labels:
    node: node3
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: smallnode1
  namespace: d8-cloud-instance-manager
  labels:
    node: smallnode1
`
		stateMachineVictim = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: victimnode
  namespace: d8-cloud-instance-manager
  labels:
    node.deckhouse.io/group: someng
    node.deckhouse.io/chaos-monkey-victim: ""
    node: victimnode
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	for _, gIsNgCloud := range []bool{true, false} {
		Context(fmt.Sprintf("Cloud: %t :: ", gIsNgCloud), func() {
			isNgCloud := gIsNgCloud

			stateNGSmall := ""
			stateNGLarge := ""
			stateNGLargeBroken := ""
			if isNgCloud {
				stateNGSmall = stateCloudNGSmall
				stateNGLarge = stateCloudNGLarge
				stateNGLargeBroken = stateCloudNGLargeBroken
			} else {
				stateNGSmall = stateHybridNGSmall
				stateNGLarge = stateHybridNGLarge
				stateNGLargeBroken = stateHybridNGLargeBroken
			}

			Context("Cluster with ngs ready for chaos", func() {
				BeforeEach(func() {
					f.KubeStateSet(stateNGSmall + stateNGLarge + stateNodes + stateMachines)
					f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
					f.AddHookEnv("D8_TEST_RANDOM_SEED=11")
					f.RunHook()
				})

				It("Hook is lucky to run monkey. One machine must be deleted.", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeFalse())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
				})
			})

			Context("Cluster with broken large ng", func() {
				BeforeEach(func() {
					f.KubeStateSet(stateNGSmall + stateNGLargeBroken + stateNodes + stateMachines)
					f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
					f.AddHookEnv("D8_TEST_RANDOM_SEED=11")
					f.RunHook()
				})

				It("Hook is lucky to run monkey. All machines must survive.", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
				})
			})

			Context("Cluster with large ready ng and victim machine", func() {
				BeforeEach(func() {
					f.KubeStateSet(stateNGSmall + stateNGLarge + stateNodes + stateMachines + stateMachineVictim)
					f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
					f.AddHookEnv("D8_TEST_RANDOM_SEED=11")
					f.RunHook()
				})

				It("Hook is lucky to run monkey. All machines must survive.", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
				})
			})

			Context("Hook isn't lucky to run monkey. All machines must survive.", func() {
				BeforeEach(func() {
					f.KubeStateSet(stateNGSmall + stateNGLarge + stateNodes + stateMachines)
					f.BindingContexts.Set(f.GenerateScheduleContext("* * * * *"))
					f.AddHookEnv("D8_TEST_RANDOM_SEED=0")
					f.RunHook()
				})

				It("", func() {
					Expect(f).To(ExecuteSuccessfully())

					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node1").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node2").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "node3").Exists()).To(BeTrue())
					Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "smallnode1").Exists()).To(BeTrue())
				})
			})
		})
	}
})
