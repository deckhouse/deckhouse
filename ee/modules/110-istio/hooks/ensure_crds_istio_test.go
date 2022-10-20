/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: istio :: hooks :: ensure_crds_istio ::", func() {
	f := HookExecutionConfigInit(`{ "istio": { "internal": { } } }`, `{"istio":{}}`)

	Context("Empty cluster, no globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("istio.internal.globalVersion value isn't discovered by discovery_versions.go yet"))
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeFalse())
		})
	})

	Context("Only globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.991 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))
		})
	})

	Context("globalVersion in values and additionalVersion older than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["0.990"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.992 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.991"))
		})
	})

	Context("globalVersion in values and additionalVersion newer than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "0.991")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["0.992"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v0.992 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("0.992"))
		})
	})
})
