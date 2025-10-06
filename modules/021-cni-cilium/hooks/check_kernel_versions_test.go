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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
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
    kernelVersion: 5.8.0-90-generic
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
		stateNode4 = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-4
  labels:
    node.deckhouse.io/group: test
status:
  nodeInfo:
    kernelVersion: 2.0.0-10-generic
`
	)

	f := HookExecutionConfigInit(`{"cniCilium": { "internal":{}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Values cniCilium.internal.minimalRequiredKernelVersionConstraint is set when only cni-cilium enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium]"))
			f.ValuesSet("cniCilium.internal.minimalRequiredKernelVersionConstraint", ">= 4.9.17")
			f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3))

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f.ValuesGet("cniCilium.internal.minimalRequiredKernelVersionConstraint").String()).To(Equal(">= 4.9.17"))
		})
	})

	Context("Module cni-cilium with extraLoadBalancerAlgorithmsEnabled parameter is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium]"))
			f.ValuesSet("cniCilium.internal.minimalRequiredKernelVersionConstraint", ">= 4.9.17")
			f.ValuesSet("cniCilium.extraLoadBalancerAlgorithmsEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3))

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f.ValuesGet("cniCilium.internal.minimalRequiredKernelVersionConstraint").String()).To(Equal(">= 5.15.0"))
		})
	})

	Context("Values cniCilium.internal.minimalRequiredKernelVersionConstraint is set when cni-cilium,istio,openvpn enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium, istio, openvpn]"))
			f.ValuesSet("cniCilium.internal.minimalRequiredKernelVersionConstraint", ">= 5.8")
			f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3))

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f.ValuesGet("cniCilium.internal.minimalRequiredKernelVersionConstraint").String()).To(Equal(">= 5.8"))
		})
	})

	Context("Finding the minimum kernel version of cluster nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3 + stateNode4))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			currentMinimalLinuxKernelVersion, _ := requirements.GetValue("currentMinimalLinuxKernelVersion")
			Expect(currentMinimalLinuxKernelVersion).To(Equal("2.0.0-10-generic"))
		})
	})

	Context("Cilium, istio, openvpn modules enabled, nodes with proper kernels", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium, istio, openvpn]"))
			f.ValuesSet("cniCilium.internal.minimalRequiredKernelVersionConstraint", ">= 5.8")
			f.BindingContexts.Set(f.KubeStateSet(stateNode3))

			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cilium, istio, openvpn modules enabled, added node with improper kernel", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", []byte("[cni-cilium, istio, openvpn]"))
			f.ValuesSet("cniCilium.internal.minimalRequiredKernelVersionConstraint", ">= 5.8")
			f.BindingContexts.Set(f.KubeStateSet(stateNode1 + stateNode2 + stateNode3))

			f.RunHook()
		})

		It("Hook must execute successfully, metric must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[1].Labels).To(Equal(map[string]string{
				"affected_module": "cni-cilium",
				"constraint":      ">= 5.8",
				"node":            "node-2",
				"kernel_version":  "3.10.0-1127.el7.x86_64",
			}))
		})
	})
})
