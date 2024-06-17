/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: system-registry :: hooks :: set_init_configuration_test ::", func() {
	const (
		initSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: system-registry-init-configuration
  namespace: d8-system
type: Opaque
data:
  registryMode: UHJveHkK
  upstreamRegistryAddress: cmVnaXN0cnkuZXhhbXBsZS5pbwo=
  upstreamRegistryAuth: ZFhObGNqcHdZWE56ZDI5eVpBbz0K
  upstreamRegistryCA: Cg==
  upstreamRegistryPath: L3Rlc3QvcGF0aAo=
  upstreamRegistryScheme: aHR0cAo=
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Create module config by secret", func() {

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Empty secret", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Someone added system-registry-init-configuration", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(initSecret))
				f.RunHook()
			})

			It("ModuleConfig/system-registry must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())

				// Expected module config
				moduleConfig := f.KubernetesResource("ModuleConfig", "", "system-registry")
				Expect(moduleConfig.Exists()).To(BeTrue())
				Expect(moduleConfig.ToYaml()).To(MatchYAML(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: system-registry
  creationTimestamp: null
spec:
  version: 1
  enabled: true
  settings:
    registryMode: Proxy
    upstreamRegistry:
      upstreamRegistryHost: registry.example.io
      upstreamRegistryScheme: http
      upstreamRegistryCa: ""
      upstreamRegistryPath: /test/path
      upstreamRegistryUser: user
      upstreamRegistryPassword: password
status:
  message: ""
  version: ""
`))
				// Unexpected secret
				unexpInitSecret := f.KubernetesResource("Secret", "d8-system", "system-registry-init-configuration")
				Expect(unexpInitSecret.Exists()).To(BeFalse())
			})
		})
	})

	Context("Update module config by secret", func() {

		const (
			initModuleConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: system-registry
  creationTimestamp: null
spec:
  version: 1
  enabled: true
  settings:
    upstreamRegistry:
      upstreamRegistryHost: null
      upstreamRegistryCa: null
      upstreamRegistryPath: /test/path
      upstreamRegistryUser: user
      upstreamRegistryPassword: "1234567890"
	`
		)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Empty secret", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Someone added system-registry-init-configuration", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(initSecret + "\n" + initModuleConfig))
				f.RunHook()
			})

			It("ModuleConfig/system-registry must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())

				// Expected module config
				moduleConfig := f.KubernetesResource("ModuleConfig", "", "system-registry")
				Expect(moduleConfig.Exists()).To(BeTrue())
				Expect(moduleConfig.ToYaml()).To(MatchYAML(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: system-registry
  creationTimestamp: null
spec:
  version: 1
  enabled: true
  settings:
    registryMode: Proxy
    upstreamRegistry:
      upstreamRegistryHost: registry.example.io
      upstreamRegistryScheme: http
      upstreamRegistryCa: ""
      upstreamRegistryPath: /test/path
      upstreamRegistryUser: user
      upstreamRegistryPassword: "1234567890"
status:
  message: ""
  version: ""
`))
				// Unexpected secret
				unexpInitSecret := f.KubernetesResource("Secret", "d8-system", "system-registry-init-configuration")
				Expect(unexpInitSecret.Exists()).To(BeFalse())
			})
		})
	})
})
