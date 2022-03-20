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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: choose nodes for ebpf exporter ::", func() {
	f := HookExecutionConfigInit("", "")
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("0 node cluster", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("3 node cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test-managed-kernel
spec:
  operatingSystem:
    manageKernel: false
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
---
apiVersion: v1
kind: Node
metadata:
  name: test-ubuntu-kernel-5.4
  labels:
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 5.4.0-54-generic
---
apiVersion: v1
kind: Node
metadata:
  name: test-ubuntu-kernel-4.9
  labels:
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 4.9.0-51-generic
---
apiVersion: v1
kind: Node
metadata:
  name: test-ubuntu-kernel-4.9-labeled
  labels:
    monitoring-kubernetes.deckhouse.io/ebpf-supported: ""
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 4.9.0-51-generic
---
apiVersion: v1
kind: Node
metadata:
  name: test-ubuntu-kernel-5.4-labeled-ng-managed-kernel
  labels:
    monitoring-kubernetes.deckhouse.io/ebpf-supported: ""
    node.deckhouse.io/group: test-managed-kernel
status:
  nodeInfo:
    kernelVersion: 5.4.0-54-generic
`, 3))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should label or unlabel nodes", func() {
			Expect(f).To(ExecuteSuccessfully())

			ubuntu54Node := f.KubernetesGlobalResource("Node", "test-ubuntu-kernel-5.4")
			ubuntu49Node := f.KubernetesGlobalResource("Node", "test-ubuntu-kernel-4.9")
			unknown49NodeLabeled := f.KubernetesGlobalResource("Node", "test-ubuntu-kernel-4.9-labeled")
			ubuntu54NodeWithManagedKernel := f.KubernetesGlobalResource("Node", "test-ubuntu-kernel-5.4-labeled-ng-managed-kernel")

			Expect(ubuntu54Node.Exists()).To(BeTrue())
			Expect(ubuntu49Node.Exists()).To(BeTrue())
			Expect(unknown49NodeLabeled.Exists()).To(BeTrue())
			Expect(ubuntu54NodeWithManagedKernel.Exists()).To(BeTrue())

			Expect(ubuntu54Node.Field("metadata.labels").Map()).To(HaveKey(ebpfSchedulingLabelKey))
			Expect(ubuntu49Node.Field("metadata.labels").Map()).To(Not(HaveKey(ebpfSchedulingLabelKey)))
			Expect(unknown49NodeLabeled.Field("metadata.labels").Map()).To(Not(HaveKey(ebpfSchedulingLabelKey)))
			Expect(ubuntu54NodeWithManagedKernel.Field("metadata.labels").Map()).To(Not(HaveKey(ebpfSchedulingLabelKey)))
		})
	})
})
