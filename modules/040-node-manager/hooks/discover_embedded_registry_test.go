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

// #TODO change to all system-registry to embedded-registry
var _ = Describe("Modules :: node-manager :: hooks :: discover_embedded_registry ::", func() {
	const (
		stateDeckhouseEmbeddedRegistryPod1 = `
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
		stateDeckhouseEmbeddedRegistryPod2 = `
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
		stateDeckhouseEmbeddedRegistryPod3 = `
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
		stateRegistryPkiSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    module: embedded-registry
    type: ca-secret
  name: registry-pki
  namespace: d8-system
type: Opaque
data:
  registry-ca.crt: Y2FfY2VydA==  # base64("ca_cert")
  registry-ca.key: Y2Ffa2V5      # base64("ca_key")
`
		stateRegistryUserRoSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    module: embedded-registry
  name: registry-user-ro
  namespace: d8-system
type: Opaque
data:
  name: dXNlcg==          # base64("user")
  password: cGFzc3dvcmQ=  # base64("password")
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"systemRegistry":{}}}}`, `{}`)

	Context("embedded registry pods are not found", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
		It("`nodeManager.internal.systemRegistry.registryCA` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.registryCA").Exists()).To(BeFalse())
		})

		It("`nodeManager.internal.systemRegistry.auth.username` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.username").Exists()).To(BeFalse())
		})

		It("`nodeManager.internal.systemRegistry.auth.password` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.password").Exists()).To(BeFalse())
		})
		It("`nodeManager.internal.systemRegistry.address` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").Exists()).To(BeFalse())
		})
	})

	Context("One embedded registry pod, both secrets are present", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseEmbeddedRegistryPod1 + stateRegistryPkiSecret + stateRegistryUserRoSecret))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`nodeManager.internal.systemRegistry.addresses` must be ['192.168.199.233:5001']", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.addresses").String()).To(MatchJSON(`["192.168.199.233:5001"]`))
		})

		It("`nodeManager.internal.systemRegistry.address` must be 'embedded-registry.d8-system.svc:5001'", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").String()).To(Equal("embedded-registry.d8-system.svc:5001"))
		})

		Context("Add second embedded registry pod", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseEmbeddedRegistryPod1 + stateDeckhouseEmbeddedRegistryPod2 + stateRegistryPkiSecret + stateRegistryUserRoSecret))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistry.addresses` must be ['192.168.199.233:5001','192.168.199.234:5001']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.addresses").String()).To(MatchJSON(`["192.168.199.233:5001","192.168.199.234:5001"]`))
			})

			It("`nodeManager.internal.systemRegistry.address` must be 'embedded-registry.d8-system.svc:5001'", func() {
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").String()).To(Equal("embedded-registry.d8-system.svc:5001"))
			})

			Context("Add third embedded registry pod", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseEmbeddedRegistryPod1 + stateDeckhouseEmbeddedRegistryPod2 + stateDeckhouseEmbeddedRegistryPod3 + stateRegistryPkiSecret + stateRegistryUserRoSecret))
					f.RunHook()
				})

				It("`nodeManager.internal.systemRegistry.addresses` must be ['192.168.199.233:5001','192.168.199.234:5001','192.168.199.235:5001']", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.addresses").String()).To(MatchJSON(`["192.168.199.233:5001","192.168.199.234:5001","192.168.199.235:5001"]`))
				})

				It("`nodeManager.internal.systemRegistry.address` must be 'embedded-registry.d8-system.svc:5001'", func() {
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").String()).To(Equal("embedded-registry.d8-system.svc:5001"))
				})
			})
		})
	})

	Context("One embedded registry pod, secrets are missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseEmbeddedRegistryPod1))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("`nodeManager.internal.systemRegistry.addresses` must be ['192.168.199.233:5001']", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.addresses").String()).To(MatchJSON(`["192.168.199.233:5001"]`))
		})

		It("`nodeManager.internal.systemRegistry.registryCA` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.registryCA").Exists()).To(BeFalse())
		})

		It("`nodeManager.internal.systemRegistry.auth.username` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.username").Exists()).To(BeFalse())
		})

		It("`nodeManager.internal.systemRegistry.auth.password` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.password").Exists()).To(BeFalse())
		})

		It("`nodeManager.internal.systemRegistry.address` should not be set", func() {
			Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").Exists()).To(BeFalse())
		})

		Context("Add second embedded registry pod", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseEmbeddedRegistryPod1 + stateDeckhouseEmbeddedRegistryPod2))
				f.RunHook()
			})

			It("`nodeManager.internal.systemRegistry.addresses` must be ['192.168.199.233:5001','192.168.199.234:5001']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.addresses").String()).To(MatchJSON(`["192.168.199.233:5001","192.168.199.234:5001"]`))
			})
			It("`nodeManager.internal.systemRegistry.registryCA` should not be set", func() {
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.registryCA").Exists()).To(BeFalse())
			})

			It("`nodeManager.internal.systemRegistry.auth.username` should not be set", func() {
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.username").Exists()).To(BeFalse())
			})

			It("`nodeManager.internal.systemRegistry.auth.password` should not be set", func() {
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.password").Exists()).To(BeFalse())
			})
			It("`nodeManager.internal.systemRegistry.address` should not be set", func() {
				Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").Exists()).To(BeFalse())
			})

			Context("Add secrets", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateDeckhouseEmbeddedRegistryPod1 + stateDeckhouseEmbeddedRegistryPod2 + stateRegistryPkiSecret + stateRegistryUserRoSecret))
					f.RunHook()
				})

				It("`nodeManager.internal.systemRegistry.addresses` must be ['192.168.199.233:5001','192.168.199.234:5001']", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.addresses").String()).To(MatchJSON(`["192.168.199.233:5001","192.168.199.234:5001"]`))
				})

				It("`nodeManager.internal.systemRegistry.registryCA` must be 'ca_cert'", func() {
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.registryCA").String()).To(Equal("ca_cert"))
				})

				It("`nodeManager.internal.systemRegistry.auth.username` must be 'user'", func() {
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.username").String()).To(Equal("user"))
				})

				It("`nodeManager.internal.systemRegistry.auth.password` must be 'password'", func() {
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.auth.password").String()).To(Equal("password"))
				})

				It("`nodeManager.internal.systemRegistry.address` must be 'embedded-registry.d8-system.svc:5001'", func() {
					Expect(f.ValuesGet("nodeManager.internal.systemRegistry.address").String()).To(Equal("embedded-registry.d8-system.svc:5001"))
				})
			})
		})
	})
})
