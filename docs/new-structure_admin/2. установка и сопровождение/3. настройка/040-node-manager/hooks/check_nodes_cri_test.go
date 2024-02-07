// Copyright 2023 Flant JSC
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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	ngCriNotManagedKubeVer1_25 = `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ngNotManaged
spec:
  cri:
    type: NotManaged
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ngNotManaged
status:
  nodeInfo:
    containerRuntimeVersion: docker
    kubeletVersion: v1.25.0
`

	nodeWithoutContainerVersion = `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: group
status:
  nodeInfo:
    kubeletVersion: v1.29.1
`

	nodeContainerd = `
---
apiVersion: v1
kind: Node
metadata:
  name: node1.1
  labels:
    node.deckhouse.io/group: group
status:
  nodeInfo:
    containerRuntimeVersion: containerd
    kubeletVersion: v1.29.1
`
)

var _ = Describe("node-manager :: check_containerd_nodes ", func() {
	f := HookExecutionConfigInit(`{"global": {"clusterConfiguration": {"apiVersion": "deckhouse.io/v1", "clusterType": "Static", "kind": "ClusterConfiguration", "kubernetesVersion": "1.29", "podSubnetCIDR": "10.111.0.0/16", "serviceSubnetCIDR": "10.222.0.0/16"}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Nodes objects are not found", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.defaultCRI", criTypeContainerd)
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It(hasNodesWithDocker+" should not exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			_, exists := requirements.GetValue(hasNodesWithDocker)
			Expect(exists).To(BeFalse())
		})
	})

	Context("One node without status.nodeInfo.containerRuntimeVersion set and defaultCRI is "+criTypeContainerd, func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.defaultCRI", criTypeContainerd)
			f.BindingContexts.Set(
				f.KubeStateSet(nodeWithoutContainerVersion),
			)
			f.RunHook()
		})

		It(hasNodesWithDocker+" should exist and false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesWithDocker)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("One node without status.nodeInfo.containerRuntimeVersion set and defaultCRI is "+criTypeDocker, func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.defaultCRI", criTypeDocker)
			f.BindingContexts.Set(f.KubeStateSet(nodeWithoutContainerVersion))
			f.RunHook()
		})

		It(hasNodesWithDocker+" should exist and true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesWithDocker)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("One node with containerD", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.defaultCRI", criTypeContainerd)
			f.BindingContexts.Set(f.KubeStateSet(nodeContainerd))
			f.RunHook()
		})

		It(hasNodesWithDocker+" should exist and false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesWithDocker)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("Node with containerD and unknownVersion", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.defaultCRI", criTypeContainerd)
			f.BindingContexts.Set(
				f.KubeStateSet(nodeContainerd + nodeWithoutContainerVersion),
			)
			f.RunHook()
		})

		It(hasNodesWithDocker+" should exist and false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesWithDocker)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("NodeGroup with CRI NotManaged, max kube ver 1.25.0", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.defaultCRI", criTypeContainerd)
			f.BindingContexts.Set(
				f.KubeStateSetAndWaitForBindingContexts(ngCriNotManagedKubeVer1_25, 1),
			)
			f.RunHook()
		})
		It("Max kube version > "+notManagedCriMaxKubeVersion, func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesWithDocker)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})
})
