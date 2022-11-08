/*
Copyright 2022 Flant JSC

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

	f := HookExecutionConfigInit(`{"deckhouse": { "internal":{}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cilium, istio, openvpn modules enabled, nodes with proper kernels", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium, istio, openvpn]"))
			f.BindingContexts.Set(f.KubeStateSet(stateNode3))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Cilium, istio, openvpn modules enabled, added node with improper kernel", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3))
				f.RunHook()
			})

			It("Hook must execute successfully, metric must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(6))
				Expect(m[1].Labels).To(Equal(map[string]string{
					"affected_module": "cni-cilium",
					"constraint":      ">= 4.9.17",
					"node":            "node-2",
					"kernel_version":  "3.10.0-1127.el7.x86_64",
				}))
				Expect(m[2].Labels).To(Equal(map[string]string{
					"affected_module": "cni-cilium,istio",
					"constraint":      ">= 5.7",
					"node":            "node-1",
					"kernel_version":  "5.4.0-90-generic",
				}))
				Expect(m[3].Labels).To(Equal(map[string]string{
					"affected_module": "cni-cilium,istio",
					"constraint":      ">= 5.7",
					"node":            "node-2",
					"kernel_version":  "3.10.0-1127.el7.x86_64",
				}))
				Expect(m[4].Labels).To(Equal(map[string]string{
					"affected_module": "cni-cilium,openvpn",
					"constraint":      ">= 5.7",
					"node":            "node-1",
					"kernel_version":  "5.4.0-90-generic",
				}))
				Expect(m[5].Labels).To(Equal(map[string]string{
					"affected_module": "cni-cilium,openvpn",
					"constraint":      ">= 5.7",
					"node":            "node-2",
					"kernel_version":  "3.10.0-1127.el7.x86_64",
				}))
			})
		})
	})
})
