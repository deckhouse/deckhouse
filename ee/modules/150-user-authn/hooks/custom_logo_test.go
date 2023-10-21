/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: set custom logo for dex", func() {
	f := HookExecutionConfigInit(`{"global": {}, "userAuthn": {"internal": {"customLogo": {}}}}`, `{}`)
	Context("ConfigMap with logo in d8-system does not exist", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run and set customLogo enabled value to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.customLogo.enabled").Bool()).To(BeFalse())
		})
	})

	Context("ConfigMap with logo in d8-system exists", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: whitelabel-custom-logo
  namespace: d8-system
data:
  dexLogo: svg
  dexTitle: svg
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run and set customLogo enabled value to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.customLogo.enabled").Bool()).To(BeTrue())
			cm := f.KubernetesResource("ConfigMap", ns, cmName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.dexLogo").String()).ToNot(BeEmpty())
			Expect(cm.Field("data.dexTitle").String()).ToNot(BeEmpty())
		})

		Context("ConfigMap was deleted", func() {
			BeforeEach(func() {
				f.KubeStateSet(``)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("Hook should run and set customLogo value to false. CM must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.customLogo.enabled").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("ConfigMap", ns, cmName).Exists()).To(BeFalse())
			})
		})
	})

	Context("ConfigMap with logo in d8-system exists but does not have dexLogo", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: whitelabel-custom-logo
  namespace: d8-system
data:
  grafanaLogo: svg
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run and set customLogo enabled value to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.customLogo.enabled").Bool()).To(BeFalse())
		})
	})
})
