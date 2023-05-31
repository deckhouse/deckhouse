// Copyright 2022 Flant JSC
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
	nodeWithoutContainerVersion = `
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: group
`

	nodeContainerD = `
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
`
	nodeDocker = `
---
apiVersion: v1
kind: Node
metadata:
  name: node2
  labels:
    node.deckhouse.io/group: group
status:
  nodeInfo:
    containerRuntimeVersion: docker
`
	nodeUnknownVersion = `
---
apiVersion: v1
kind: Node
metadata:
  name: node3
  labels:
    node.deckhouse.io/group: group
status:
  nodeInfo:
    containerRuntimeVersion: foo
`
)

var _ = Describe("node-manager :: check_containerd_nodes ", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Nodes objects are not found", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should have no a container engine type check", func() {
			Expect(f).To(ExecuteSuccessfully())
			_, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeFalse())
		})
	})

	Context("One node without status.nodeInfo.containerRuntimeVersion set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeWithoutContainerVersion))
			f.RunHook()
		})

		It("Should have a container engine type check true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("One node with containerD", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeContainerD))
			f.RunHook()
		})

		It("Should have a container engine type check false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("Node with containerD and unknownVersion", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeContainerD + nodeWithoutContainerVersion))
			f.RunHook()
		})

		It("Should have a container engine type check true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("Node with containerD and docker", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodeContainerD + nodeDocker))
			f.RunHook()
		})

		It("Should have a container engine type check true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("Node with containerD and docker and unknownVersion", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(nodeContainerD + nodeDocker + nodeUnknownVersion),
			)
			f.RunHook()
		})

		It("Should have a container engine type check true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("Node with containerD and docker and unknownVersion and node without status.nodeInfo.containerRuntimeVersion set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(
					nodeContainerD +
						nodeDocker +
						nodeUnknownVersion +
						nodeWithoutContainerVersion,
				),
			)
			f.RunHook()
		})

		It("Should have a container engine type check true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(hasNodesOtherThanContainerD)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})
})
