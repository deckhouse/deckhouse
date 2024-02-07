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

var _ = Describe("User Authn hooks :: discover dex clusterIP ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{}, "https": {"mode":"CertManager"}}}`, "")

	Context("With dex service without clusterIP", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Service
metadata:
  name: dex
  namespace: d8-user-authn
spec:
  clusterIP: None
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should delete service", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Service", "d8-user-authn", "dex").Exists()).To(BeFalse())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexClusterIP").Exists()).To(BeFalse())
		})
	})

	Context("With dex service with clusterIP occurred", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Service
metadata:
  name: dex
  namespace: d8-user-authn
spec:
  clusterIP: 1.2.3.4
`))
			f.RunHook()
		})

		It("Should save it to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexClusterIP").String()).To(Equal("1.2.3.4"))
		})
	})

	Context("With dex service with clusterIP before helm", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Service
metadata:
  name: dex
  namespace: d8-user-authn
spec:
  clusterIP: 1.2.3.4
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should save it to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexClusterIP").String()).To(Equal("1.2.3.4"))
		})
	})
})
