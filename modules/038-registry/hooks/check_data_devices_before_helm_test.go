/*
Copyright 2025 Flant JSC

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

var _ = Describe("Modules :: registry :: hooks :: handle registry data device nodes", func() {
	const initValues = `
global:
  clusterConfiguration:
    apiVersion: deckhouse.io/v1alpha1
    cloud:
      prefix: sandbox
      provider: OpenStack
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "1.29"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
registry:
  internal:
    orchestrator:
      state:
        target_mode: "Local"
`

	f := HookExecutionConfigInit(initValues, `{}`)

	Context("With Cloud cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
		})

		Context("With nodes having registry data device labels", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: cloud-node-001
  labels:
    node.deckhouse.io/registry-data-device-ready: "true"
`, 1))
				f.RunHook()
			})

			It("Should correctly identify nodes with registry data device labels in Cloud cluster", func() {
				Expect(f).To(ExecuteSuccessfully())

				node := f.KubernetesResource("Node", "", "cloud-node-001")
				Expect(node.Exists()).To(BeTrue())
			})
		})

		Context("With nodes not having registry data device labels", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: cloud-node-002
`, 1))
				f.RunHook()
			})

			It("Should fail and report absence of nodes with registry data device labels in Cloud cluster", func() {
				Expect(f).ToNot(ExecuteSuccessfully())

				node := f.KubernetesResource("Node", "", "cloud-node-002")
				Expect(node.Exists()).To(BeTrue())
			})
		})

		Context("No nodes present in Cloud cluster", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
				f.RunHook()
			})

			It("Should correctly handle absence of nodes in Cloud cluster and throw an error", func() {
				Expect(f).ToNot(ExecuteSuccessfully())

				node := f.KubernetesResource("Node", "", "missing-cloud-node")
				Expect(node.Exists()).To(BeFalse())
			})
		})
	})

	Context("With Static cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.clusterType", "Static")
		})

		Context("Static cluster nodes without registry readiness labels", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: static-node-003
`, 1))
				f.RunHook()
			})

			It("Should include nodes of cluster type Static even without registry readiness labels", func() {
				Expect(f).To(ExecuteSuccessfully())

				node := f.KubernetesResource("Node", "", "static-node-003")
				Expect(node.Exists()).To(BeTrue())
			})
		})

		Context("No nodes present in Static cluster", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
				f.RunHook()
			})

			It("Should correctly report absence of nodes in Static cluster without errors", func() {
				Expect(f).To(ExecuteSuccessfully())

				node := f.KubernetesResource("Node", "", "missing-static-node")
				Expect(node.Exists()).To(BeFalse())
			})
		})
	})

	Context("With Cloud cluster without devices and in Direct mode", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
			f.ValuesSet("registry.internal.orchestrator.state.target_mode", "Direct")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: cloud-node-001
`, 1))
			f.RunHook()
		})

		It("Should correctly identify nodes with registry data device labels in Cloud cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("With Cloud cluster without devices and in Unmanaged mode", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
			f.ValuesSet("registry.internal.orchestrator.state.target_mode", "Unmanaged")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  name: cloud-node-001
`, 1))
			f.RunHook()
		})

		It("Should correctly identify nodes with registry data device labels in Cloud cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
