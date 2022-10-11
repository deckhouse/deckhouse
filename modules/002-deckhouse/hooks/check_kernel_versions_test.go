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

var _ = Describe("Modules :: deckhouse :: hooks :: check_kernel_versions ::", func() {
	const (
		stateNode1 = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
  labels:
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 5.4.0-90-generic
`
		stateNode2 = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
  labels:
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 3.10.0-1127.el7.x86_64
`

		stateNode3 = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-3
  labels:
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 5.15.0-10-generic
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cilium module enabled, nodes with proper kernels", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium]"))
			f.KubeStateSet(stateNode1 + stateNode3)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Cilium module enabled, added node with improper kernel", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3))
				f.RunHook()
			})

			It("Hook should fail, metric should be set", func() {
				Expect(f).To(Not(ExecuteSuccessfully()))
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(1))
				Expect(m[0].Labels).To(Equal(map[string]string{
					"module":         "cilium",
					"constraint":     ">= 4.9.17",
					"name":           "node-2",
					"kernel_version": "3.10.0-1127.el7.x86_64",
				}))
			})
		})
	})

	Context("Cilium and istio modules enabled, nodes with proper kernels", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium, istio]"))
			f.KubeStateSet(stateNode3)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Cilium and ision modules enabled, added node with improper kernel", func() {
			BeforeEach(func() {
				f.KubeStateSet(stateNode1 + stateNode2 + stateNode3)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("Hook should fail, metric should be set", func() {
				Expect(f).To(Not(ExecuteSuccessfully()))
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(1))
				Expect(m[0].Labels).To(Equal(map[string]string{
					"module":         "cilium,istio",
					"constraint":     ">= 5.7",
					"name":           "node-1",
					"kernel_version": "5.4.0-90-generic",
				}))
			})
		})
	})
})
