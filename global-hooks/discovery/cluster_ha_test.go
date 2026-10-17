// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. Hook must count Nodes labeled node-role.kubernetes.io/control-plane and save the number to global.discovery.clusterMasterCount,
2. Hook must count the same Nodes excluding the cordoned ones and save the number to global.discovery.clusterSchedulableMasterCount,
3. If the number of control-plane Nodes is more than one — hook must set global.discovery.clusterControlPlaneIsHighlyAvailable to true, else — to false.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cluster_ha ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateFirstMasterNode = `
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""`

		stateSecondMasterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""`

		stateThirdMasterNodeCordoned = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-2
  labels:
    node-role.kubernetes.io/control-plane: ""
spec:
  unschedulable: true`

		stateSecondMasterNodeCordoned = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
spec:
  unschedulable: true`

		stateFourthMasterNodeTainted = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-3
  labels:
    node-role.kubernetes.io/control-plane: ""
spec:
  taints:
  - key: node-role.kubernetes.io/control-plane
    effect: NoSchedule`

		stateWorkerNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
  labels:
    node.deckhouse.io/group: worker`
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

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` and `global.discovery.clusterSchedulableMasterCount` must be 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("0"))
			Expect(f.ValuesGet("global.discovery.clusterSchedulableMasterCount").String()).To(Equal("0"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})
	})

	Context("One master node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode))
			f.RunHook()
		})

		It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be false; `global.discovery.clusterMasterCount` and `global.discovery.clusterSchedulableMasterCount` must be 1", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("1"))
			Expect(f.ValuesGet("global.discovery.clusterSchedulableMasterCount").String()).To(Equal("1"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeFalse())

		})

		Context("Two master nodes in cluster", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode))
				f.RunHook()
			})

			It("`global.discovery.clusterControlPlaneIsHighlyAvailable` must be true; `global.discovery.clusterMasterCount` and `global.discovery.clusterSchedulableMasterCount` must be 2", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterSchedulableMasterCount").String()).To(Equal("2"))
				Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
			})
		})
	})

	Context("Three master nodes in cluster, one of them cordoned", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateSecondMasterNode + stateThirdMasterNodeCordoned))
			f.RunHook()
		})

		It("`global.discovery.clusterMasterCount` must be 3; `global.discovery.clusterSchedulableMasterCount` must be 2", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("3"))
			Expect(f.ValuesGet("global.discovery.clusterSchedulableMasterCount").String()).To(Equal("2"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})
	})

	Context("Every master node in cluster is cordoned", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecondMasterNodeCordoned + stateThirdMasterNodeCordoned))
			f.RunHook()
		})

		It("`global.discovery.clusterMasterCount` must be 2; `global.discovery.clusterSchedulableMasterCount` must be 0; the cluster stays highly available", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
			Expect(f.ValuesGet("global.discovery.clusterSchedulableMasterCount").String()).To(Equal("0"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})
	})

	Context("Master node with the control-plane taint next to a worker node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstMasterNode + stateFourthMasterNodeTainted + stateWorkerNode))
			f.RunHook()
		})

		It("a taint does not make a master unschedulable; a node without the control-plane label never enters the snapshot", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.clusterMasterCount").String()).To(Equal("2"))
			Expect(f.ValuesGet("global.discovery.clusterSchedulableMasterCount").String()).To(Equal("2"))
			Expect(f.ValuesGet("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool()).To(BeTrue())
		})
	})
})
