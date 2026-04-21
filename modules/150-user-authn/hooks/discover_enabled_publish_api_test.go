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

var _ = Describe("User Authn hooks :: discover enabled publish API ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{}}}`, "")

	Context("Detect enabled publish API via ingress existence", func() {
		BeforeEach(func() {

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("With kubernetes-api ingress in kube-system", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  labels:
    module: control-plane-manager
  name: kubernetes-api
  namespace: kube-system
spec:
  ingressClassName: nginx
  rules:
  - host: api.example.com
    http:
      paths:
      - backend:
          service:
            name: kubernetes
            port:
              number: 443
        pathType: ImplementationSpecific
`, 2))
				f.RunHook()
			})

			It("Should set publishAPIEnabled to true", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.publishAPIEnabled").Bool()).To(Equal(true))
			})
		})

		Context("With no ingress", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 2))
				f.RunHook()
			})

			It("Should set publishAPIEnabled to false", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.publishAPIEnabled").Bool()).To(Equal(false))
			})
		})
	})

})
