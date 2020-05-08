package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: discover_modules ::", func() {
	const (
		configMap = `
---
apiVersion: v1
data:
  url: test
  ca: test
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
`
		configMapAdded = `
---
apiVersion: v1
data:
  url: testtest
  ca: testtest
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authz
  labels:
    control-plane-configurator: ""
---
apiVersion: v1
data:
  oidcIssuerURL: test
  oidcCA: test
kind: ConfigMap
metadata:
  name: cm
  namespace: d8-user-authn
  labels:
    control-plane-configurator: ""
`
	)

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully, but no values should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
		})

		Context("Someone added d8-cloud-instance-manager-cloud-provider", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(configMap))
				f.RunHook()
			})

			It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("test"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("test"))
			})
		})
	})

	Context("Secret d8-cloud-instance-manager-cloud-provider is in cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(configMap))
			f.RunHook()
		})

		It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("test"))
			Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("test"))
		})

		Context("ConfigMap was added", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(configMapAdded))
				f.RunHook()
			})

			It("controlPlaneManager.x values must be filled with data from ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").String()).To(Equal("testtest"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").String()).To(Equal("testtest"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").String()).To(Equal("test"))
				Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").String()).To(Equal("test"))
			})

			Context("ConfigMaps were deleted", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})

				It("Hook must execute successfully, and all values should be unset", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authz.webhookCA").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcIssuerURL").Exists()).ToNot(BeTrue())
					Expect(f.ValuesGet("controlPlaneManager.apiserver.authn.oidcCA").Exists()).ToNot(BeTrue())
				})
			})
		})
	})
})
