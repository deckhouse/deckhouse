/*
Copyright 2024 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: discover_packages_proxy_addresses ::", func() {
	const (
		stateDeckhousePackageProxyPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: packages-proxy-0
  namespace: d8-cloud-instance-manager
  labels:
    app: "registry-packages-proxy"
spec:
  nodeName: master-0
status:
  hostIP: 192.168.199.233
  conditions:
  - status: "True"
    type: Ready
`
		stateDeckhousePackageProxyPod2 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: packages-proxy-1
  namespace: d8-cloud-instance-manager
  labels:
    app: "registry-packages-proxy"
spec:
  nodeName: master-1
status:
  hostIP: 192.168.199.234
  conditions:
  - status: "True"
    type: Ready
`
		stateDeckhousePackageProxyPod3 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: packages-proxy-2
  namespace: d8-cloud-instance-manager
  labels:
    app: "registry-packages-proxy"
spec:
  nodeName: master-2
status:
  hostIP: 192.168.199.235
  conditions:
  - status: "False"
    type: Ready
`
		stateDeckhousePackageProxyPodWorker = `
---
apiVersion: v1
kind: Pod
metadata:
  name: packages-proxy-worker-0
  namespace: d8-cloud-instance-manager
  labels:
    app: "registry-packages-proxy"
spec:
  nodeName: worker-0
status:
  hostIP: 192.168.199.236
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeMaster0Ready = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
    node.deckhouse.io/group: "master"
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeMaster1Ready = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
    node.deckhouse.io/group: "master"
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeMaster2Ready = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-2
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
    node.deckhouse.io/group: "master"
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeMaster0NotReady = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
    node.deckhouse.io/group: "master"
status:
  conditions:
  - status: "False"
    type: Ready
`
		stateNodeMaster0ReadyWithoutNodeGroup = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeMaster0Deleting = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
    node.deckhouse.io/group: "master"
  deletionTimestamp: "2024-01-01T00:00:00Z"
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeMaster0Unschedulable = `
---
apiVersion: v1
kind: Node
metadata:
  name: master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
    node-role.kubernetes.io/master: ""
    node.deckhouse.io/group: "master"
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeWorker0Ready = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
  labels:
    node-role.kubernetes.io/worker: ""
    node.deckhouse.io/group: "worker"
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeWorker0ReadyWithoutNodeGroup = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
  labels:
    node-role.kubernetes.io/worker: ""
status:
  conditions:
  - status: "True"
    type: Ready
`
		stateNodeWorker0Unschedulable = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-0
  labels:
    node-role.kubernetes.io/worker: ""
    node.deckhouse.io/group: "worker"
spec:
  unschedulable: true
status:
  conditions:
  - status: "True"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"packagesProxy":{}}}}`, `{}`)

	Context("Registry packages pods are not found", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("One registry proxy pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0Ready + stateDeckhousePackageProxyPod))
			f.RunHook()
		})

		It("`nodeManager.internal.packagesProxyAddresses` must be ['192.168.199.233:4219']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.233:4219"]`))
		})

		Context("Add second registry proxy pod", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0Ready + stateNodeMaster1Ready + stateDeckhousePackageProxyPod + stateDeckhousePackageProxyPod2))
				f.RunHook()
			})

			It("`nodeManager.internal.packagesProxyAddresses` must be ['192.168.199.233:4219','192.168.199.234:4219']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.233:4219","192.168.199.234:4219"]`))
			})

			Context("Add third registry proxy pod", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0Ready + stateNodeMaster1Ready + stateNodeMaster2Ready + stateDeckhousePackageProxyPod + stateDeckhousePackageProxyPod2 + stateDeckhousePackageProxyPod3))
					f.RunHook()
				})

				It("`nodeManager.internal.packagesProxyAddresses` must be ['192.168.199.233:4219','192.168.199.234:4219'], third pod is not ready", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.233:4219","192.168.199.234:4219"]`))
				})
			})
		})
	})

	Context("Registry proxy pod on NotReady node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0NotReady + stateDeckhousePackageProxyPod))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("Registry proxy pod on deleting node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0Deleting + stateDeckhousePackageProxyPod))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("Registry proxy pods on deleting control-plane and ready worker", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0Deleting + stateDeckhousePackageProxyPod + stateNodeWorker0Ready + stateDeckhousePackageProxyPodWorker))
			f.RunHook()
		})

		It("Hook should execute successfully and keep only worker endpoint in fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.236:4219"]`))
		})
	})

	Context("Registry proxy pods on unschedulable control-plane and ready worker", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0Unschedulable + stateDeckhousePackageProxyPod + stateNodeWorker0Ready + stateDeckhousePackageProxyPodWorker))
			f.RunHook()
		})

		It("Hook should execute successfully and keep only worker endpoint in fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.236:4219"]`))
		})
	})

	Context("Registry proxy pods on unmanaged control-plane and ready worker", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeMaster0ReadyWithoutNodeGroup + stateDeckhousePackageProxyPod + stateNodeWorker0Ready + stateDeckhousePackageProxyPodWorker))
			f.RunHook()
		})

		It("Hook should execute successfully and keep only worker endpoint in fallback", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.236:4219"]`))
		})
	})

	Context("Registry proxy pod on node without control-plane label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeWorker0Ready + stateDeckhousePackageProxyPodWorker))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxy.addresses").String()).To(MatchJSON(`["192.168.199.236:4219"]`))
		})
	})

	Context("Registry proxy pod on node without deckhouse nodegroup label", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeWorker0ReadyWithoutNodeGroup + stateDeckhousePackageProxyPodWorker))
			f.RunHook()
		})

		It("Hook should fail to avoid stale endpoints from unmanaged nodes", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("Registry proxy pod on unschedulable node", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNodeWorker0Unschedulable + stateDeckhousePackageProxyPodWorker))
			f.RunHook()
		})

		It("Hook should fail to avoid stale endpoints from draining nodes", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})
})
