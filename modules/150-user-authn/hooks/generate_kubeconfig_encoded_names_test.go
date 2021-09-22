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

var _ = Describe("User Authn hooks :: generate kubeconfig encoded names ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{}}}`, "")

	Context("Without kubeconfig in values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})
	})

	Context("With kubeconfig in values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSetFromYaml("userAuthn.kubeconfigGenerator", []byte(`[
{"id": "kubeconfig-one", "masterURI": "127.0.0.1", "description": "test"},
{"id": "kubeconfig-two", "masterURI": "test.example.com", "description": "test2"}
]`))
			f.RunHook()
		})

		It("Should add encoded kubeconfig names", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("userAuthn.internal.kubeconfigEncodedNames").String()).To(MatchJSON(`[
"nn2wezldn5xgm2lhfvtwk3tfojqxi33sfuymx4u44scceizf", "nn2wezldn5xgm2lhfvtwk3tfojqxi33sfuy4x4u44scceizf"
]`))
		})
	})
})
