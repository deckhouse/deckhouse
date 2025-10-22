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

var _ = Describe("Istio hooks :: generate_kiali_signing_key ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{"clusterDomain":"cluster.flomaster"}},"istio":{"internal":{"kialiSigningKey":""}}}`, "")

	Context("Empty cluster; empty value; secret doesn't exist", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Leave the value unchanged, proceed to generate the secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.ValuesGet("istio.internal.kialiSigningKey").String())).To(Equal(32))
		})
	})

	Context("Signing key is in cluster; empty value", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: kiali-signing-key
  namespace: d8-istio
type: Opaque
data:
  key: "NVo4ZFFlT2l1RkpRTllQa2duMmh1bEE0M3FuRzZ0SGI=" # 5Z8dQeOiuFJQNYPkgn2hulA43qnG6tHb
`))
			f.RunHook()
		})
		It("Signing key is not changed and copied to value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.kialiSigningKey").String()).To(Equal("5Z8dQeOiuFJQNYPkgn2hulA43qnG6tHb"))
		})
	})

	Context("Wrong signing key is in cluster; empty value", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: kiali-signing-key
  namespace: d8-istio
type: Opaque
data:
  key: "d3Jvbmcta2V5" # "wrong-key", not 32 bytes len
`))
			f.RunHook()
		})
		It("Signing key must be generated anew", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(len(f.ValuesGet("istio.internal.kialiSigningKey").String())).To(Equal(32))
		})
	})
})
