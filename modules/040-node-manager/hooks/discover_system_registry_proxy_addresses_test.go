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

var _ = Describe("Modules :: node-manager :: hooks :: discover_system_registry_proxy_addresses ::", func() {
	const (
		stateDeckhouseSystemRegistryProxyPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: system-registry-0
  namespace: d8-system
  labels:
    component: "system-registry"
    tier: "control-plane"
status:
  hostIP: 192.168.199.233
  conditions:
  - status: "True"
    type: Ready
`
		stateDeckhouseSystemRegistryProxyPod2 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: system-registry-1
  namespace: d8-system
  labels:
    component: "system-registry"
    tier: "control-plane"
status:
  hostIP: 192.168.199.234
  conditions:
  - status: "True"
    type: Ready
`
		stateDeckhouseSystemRegistryProxyPod3 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: system-registry-2
  namespace: d8-system
  labels:
    component: "system-registry"
    tier: "control-plane"
status:
  hostIP: 192.168.199.235
  conditions:
  - status: "False"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"systemRegistryProxy":{}}}}`, `{}`)

	Context("System registry pods are not found", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("One system registry pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseSystemRegistryProxyPod))
			f.RunHook()
		})

		It("`nodeManager.internal.systemRegistryProxy.addresses` must be ['192.168.199.233:5001']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.systemRegistryProxy.addresses").String()).To(MatchJSON(`["192.168.199.233:5001"]`))
		})

		Context("Add second system registry pod", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseSystemRegistryProxyPod + stateDeckhouseSystemRegistryProxyPod2))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistryProxy.addresses` must be ['192.168.199.233:5001','192.168.199.234:5001']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistryProxy.addresses").String()).To(MatchJSON(`["192.168.199.233:5001","192.168.199.234:5001"]`))
			})

			Context("Add third system registry pod", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseSystemRegistryProxyPod + stateDeckhouseSystemRegistryProxyPod2 + stateDeckhouseSystemRegistryProxyPod3))
					f.RunHook()
				})

				It("`nodeManager.internal.systemRegistryProxy.addresses` must be ['192.168.199.233:5001','192.168.199.234:5001','192.168.199.235:5001']", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("nodeManager.internal.systemRegistryProxy.addresses").String()).To(MatchJSON(`["192.168.199.233:5001","192.168.199.234:5001","192.168.199.235:5001"]`))
				})
			})
		})
	})
})
