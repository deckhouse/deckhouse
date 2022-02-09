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

var _ = Describe("User Authn hooks :: dex authenticator adoption ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "DexAuthenticator", true)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("With objects", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: upmeter
  namespace: d8-upmeter
---
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: test
  namespace: test
---
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: status
  namespace: d8-upmeter
  annotations:
    test: test
`))
			f.RunHook()
		})

		It("Should patch only required objects", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("DexAuthenticator", "d8-upmeter", "upmeter").Field("metadata.labels")).To(MatchJSON(`{"app.kubernetes.io/managed-by": "Helm"}`))
			Expect(f.KubernetesResource("DexAuthenticator", "d8-upmeter", "upmeter").Field("metadata.annotations")).To(MatchJSON(`{"meta.helm.sh/release-name": "upmeter", "meta.helm.sh/release-namespace": "d8-upmeter"}`))

			Expect(f.KubernetesResource("DexAuthenticator", "test", "test").Field("metadata.labels").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("DexAuthenticator", "test", "test").Field("metadata.annotations").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("DexAuthenticator", "d8-upmeter", "status").Field("metadata.labels")).To(MatchJSON(`{"app.kubernetes.io/managed-by": "Helm"}`))
			Expect(f.KubernetesResource("DexAuthenticator", "d8-upmeter", "status").Field("metadata.annotations")).To(MatchJSON(`{"meta.helm.sh/release-name": "upmeter", "meta.helm.sh/release-namespace": "d8-upmeter", "test": "test"}`))
		})
	})
})
