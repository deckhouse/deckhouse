package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: set_provider_id_on_static_nodes ::", func() {
	const (
		stateNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-1
spec: {}
---
apiVersion: v1
kind: Node
metadata:
  name: node-2
  labels:
    cloud-instance-manager.deckhouse.io/cloud-instance-group: worker
---
apiVersion: v1
kind: Node
metadata:
  name: node-3
spec:
  providerID: ""
---
apiVersion: v1
kind: Node
metadata:
  name: node-4
spec:
  taints:
  - key: node.cloudprovider.kubernetes.io/uninitialized
---
apiVersion: v1
kind: Node
metadata:
  name: node-5
spec:
  providerID: "super-provider"
`
	)

	f := HookExecutionConfigInit(`{"cloudProviderOpenstack":{"internal":{}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster has four nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodes))
			f.RunHook()
		})

		It("node-1: set providerID; node-2: skip; node-3: set providerID; node-4: skip; node-5: set providerID", func() {
			Expect(f).To(ExecuteSuccessfully())

			// Two patches — for node-1 and node-3
			Expect(len(f.KubernetesResourcePatch.Operations)).To(Equal(2))

			Expect(f.KubernetesResource("Node", "", "node-1").Field("spec.providerID").String()).To(Equal(`static://`))
			Expect(f.KubernetesResource("Node", "", "node-2").Field("spec.providerID").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Node", "", "node-3").Field("spec.providerID").String()).To(Equal(`static://`))
			Expect(f.KubernetesResource("Node", "", "node-4").Field("spec.providerID").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Node", "", "node-5").Field("spec.providerID").String()).To(Equal(`super-provider`))
		})
	})
})
