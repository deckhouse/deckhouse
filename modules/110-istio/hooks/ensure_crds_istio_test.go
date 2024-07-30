/*
Copyright 2023 Flant JSC

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
