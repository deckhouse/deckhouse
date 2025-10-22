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
    node.deckhouse.io/group: worker
    node.deckhouse.io/type: CloudEphemeral
---
apiVersion: v1
kind: Node
metadata:
  name: node-3
  labels:
    node.deckhouse.io/group: worker
    node.deckhouse.io/type: Static
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
---
apiVersion: v1
kind: Node
metadata:
  name: node-6
  labels:
    node.deckhouse.io/group: worker
    node.deckhouse.io/type: Static
---
apiVersion: v1
kind: Node
metadata:
  name: node-7
  labels:
    node.deckhouse.io/group: worker
    node.deckhouse.io/type: CloudStatic
spec:
  providerID: ""
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

			Expect(f.KubernetesGlobalResource("Node", "node-1").Field("spec.providerID").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Node", "node-2").Field("spec.providerID").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Node", "node-3").Field("spec.providerID").String()).To(Equal(`static://`))
			Expect(f.KubernetesGlobalResource("Node", "node-4").Field("spec.providerID").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Node", "node-5").Field("spec.providerID").String()).To(Equal(`super-provider`))
			Expect(f.KubernetesGlobalResource("Node", "node-6").Field("spec.providerID").String()).To(Equal(`static://`))
			Expect(f.KubernetesGlobalResource("Node", "node-7").Field("spec.providerID").String()).To(BeEmpty())
		})
	})
})
