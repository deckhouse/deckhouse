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
	f := HookExecutionConfigInit(`{
  "istio": {
    "internal": {
      "supportedVersions": ["4.2-test.0", "4.2-test.1", "4.2-test.2"],
    },
  }
}`, `{"istio":{}}`)

	Context("Empty cluster, no globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("istio.internal.globalVersion value isn't discovered by revisions_discovery.go yet"))
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeFalse())
		})
	})

	Context("Only globalVersion in values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "4.2-test.1")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v42test1 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("4.2-test.1"))
		})
	})

	Context("globalVersion in values and additionalVersion older than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "4.2-test.1")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["4.2-test.0"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v42test1 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("4.2-test.1"))
		})
	})

	Context("globalVersion in values and additionalVersion newer than global", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ValuesSet("istio.internal.globalVersion", "4.2-test.1")
			f.ConfigValuesSetFromYaml("istio.additionalVersions", []byte(`["4.2-test.2"]`))
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook must not fail, CRD v42test1 should be created", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("CustomResourceDefinition", "testcrds.deckhouse.io").Field("spec.scope").String()).To(Equal("4.2-test.2"))
		})
	})
})
