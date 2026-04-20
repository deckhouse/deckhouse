/*
Copyright 2026 Flant JSC

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

var _ = Describe("ingress-nginx :: hooks :: discover_legacy_kruise_management ::", func() {
	const ingressNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-ingress-nginx
`

	const ingressNamespaceWithAnnotation = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-ingress-nginx
  annotations:
    ingress-nginx.deckhouse.io/force-legacy-kruise: "true"
`

	Context("when the rollback annotation is absent", func() {
		f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressNamespace))
			f.RunHook()
		})

		It("keeps legacy Kruise management disabled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.legacyKruiseManagementEnabled").Bool()).To(BeFalse())
		})
	})

	Context("when the rollback annotation is present", func() {
		f := HookExecutionConfigInit(`{"ingressNginx":{"internal":{}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(ingressNamespaceWithAnnotation))
			f.RunHook()
		})

		It("enables legacy Kruise management", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.legacyKruiseManagementEnabled").Bool()).To(BeTrue())
		})
	})
})
