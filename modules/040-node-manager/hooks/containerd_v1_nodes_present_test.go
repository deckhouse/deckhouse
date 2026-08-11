/*
Copyright 2026 Flant JSC

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// setClusterConfigurationWithDefaultCRI emulates the d8-cluster-configuration secret
// discovered into global values with the given defaultCRI.
func setClusterConfigurationWithDefaultCRI(f *HookExecutionConfig, defaultCRI string) {
	f.ValuesSetFromYaml("global.clusterConfiguration", []byte(fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterDomain: cluster.local
clusterType: Static
defaultCRI: %s
kubernetesVersion: "1.30"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`, defaultCRI)))
}

var _ = Describe("Modules :: node-manager :: hooks :: containerd_v1_nodes_present ::", func() {
	const (
		noNodes = ``

		containerdV2Node = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-v2
status:
  nodeInfo:
    containerRuntimeVersion: containerd://2.0.1
`

		containerdV1Node = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-v1
status:
  nodeInfo:
    containerRuntimeVersion: containerd://1.7.13
`

		mixedNodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: node-v2
status:
  nodeInfo:
    containerRuntimeVersion: containerd://2.0.1
---
apiVersion: v1
kind: Node
metadata:
  name: node-v1
status:
  nodeInfo:
    containerRuntimeVersion: containerd://1.7.13
`

		containerdV1NodeGroup = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-v1
spec:
  nodeType: Static
  cri:
    type: Containerd
`

		containerdV2NodeGroup = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-v2
spec:
  nodeType: Static
  cri:
    type: ContainerdV2
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("No nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(noNodes))
			f.RunHook()
		})

		It("Hook must not fail, value should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("Only containerd v2 nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(containerdV2Node))
			f.RunHook()
		})

		It("Hook must not fail, value should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("Only containerd v1 nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(containerdV1Node))
			f.RunHook()
		})

		It("Hook must not fail, value should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("Mixed containerd v1 and v2 nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(mixedNodes))
			f.RunHook()
		})

		It("Hook must not fail, value should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("NodeGroup with cri.type Containerd and no nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(containerdV1NodeGroup))
			f.RunHook()
		})

		It("Hook must not fail, value should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("NodeGroup with cri.type ContainerdV2 and containerd v2 node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(containerdV2NodeGroup + containerdV2Node))
			f.RunHook()
		})

		It("Hook must not fail, value should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("defaultCRI is Containerd and no nodes", func() {
		BeforeEach(func() {
			setClusterConfigurationWithDefaultCRI(f, "Containerd")
			f.BindingContexts.Set(f.KubeStateSet(noNodes))
			f.RunHook()
		})

		It("Hook must not fail, value should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})

	Context("defaultCRI is ContainerdV2 and no nodes", func() {
		BeforeEach(func() {
			setClusterConfigurationWithDefaultCRI(f, "ContainerdV2")
			f.BindingContexts.Set(f.KubeStateSet(noNodes))
			f.RunHook()
		})

		It("Hook must not fail, value should be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(containerdV1NodesPresentValuesKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})
})
