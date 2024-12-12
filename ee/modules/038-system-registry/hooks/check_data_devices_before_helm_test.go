/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: system-registry :: hooks :: handle registry data device nodes", func() {
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
})
