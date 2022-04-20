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

	Context("1 node cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: test-ubuntu-kernel-5.4-test-1
status:
  nodeInfo:
    kernelVersion: 5.4.0-54-generic
---
apiVersion: v1
kind: Node
metadata:
  name: test-ubuntu-kernel-5.4-test-2
  labels:
    monitoring-kubernetes.deckhouse.io/ebpf-supported: ""
status:
  nodeInfo:
    kernelVersion: 5.4.0-54-generic
`, 2))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should label or unlabel nodes", func() {
			Expect(f).To(ExecuteSuccessfully())

			test1Node := f.KubernetesGlobalResource("Node", "test-ubuntu-kernel-5.4-test-1")
			test2Node := f.KubernetesGlobalResource("Node", "test-ubuntu-kernel-5.4-test-2")

			Expect(test1Node.Exists()).To(BeTrue())
			Expect(test2Node.Exists()).To(BeTrue())

			Expect(test1Node.Field("metadata.labels").Map()).To(Not(HaveKey(deprecatedEbpfSchedulingLabelKey)))
			Expect(test2Node.Field("metadata.labels").Map()).To(Not(HaveKey(deprecatedEbpfSchedulingLabelKey)))
		})
	})
})
