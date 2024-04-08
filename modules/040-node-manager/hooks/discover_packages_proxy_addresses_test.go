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

var _ = Describe("Modules :: node-manager :: hooks :: discover_packages_proxy_addresses ::", func() {
	const (
		stateDeckhousePackageProxyPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: packages-proxy-0
  namespace: ` + packagesProxyNamespace + `
  labels:
    app: "registry-packages-proxy"
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
  namespace: ` + packagesProxyNamespace + `
  labels:
    app: "registry-packages-proxy"
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
  namespace: ` + packagesProxyNamespace + `
  labels:
    app: "registry-packages-proxy"
status:
  hostIP: 192.168.199.235
  conditions:
  - status: "False"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)

	Context("Registry packages pods are not found", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("One registry proxy pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhousePackageProxyPod))
			f.RunHook()
		})

		It("`nodeManager.internal.packagesProxyAddresses` must be ['192.168.199.233:5443']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxyAddresses").String()).To(MatchJSON(`["192.168.199.233:5443"]`))
		})

		Context("Add second registry proxy pod", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeckhousePackageProxyPod + stateDeckhousePackageProxyPod2))
				f.RunHook()
			})

			It("`nodeManager.internal.packagesProxyAddresses` must be ['192.168.199.233:5443','192.168.199.234:5443']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.packagesProxyAddresses").String()).To(MatchJSON(`["192.168.199.233:5443","192.168.199.234:5443"]`))
			})

			Context("Add third registry proxy pod", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateDeckhousePackageProxyPod + stateDeckhousePackageProxyPod2 + stateDeckhousePackageProxyPod3))
					f.RunHook()
				})

				It("`nodeManager.internal.packagesProxyAddresses` must be ['192.168.199.233:5443','192.168.199.234:5443'], third pod is not ready", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("nodeManager.internal.packagesProxyAddresses").String()).To(MatchJSON(`["192.168.199.233:5443","192.168.199.234:5443"]`))
				})
			})
		})
	})
})
