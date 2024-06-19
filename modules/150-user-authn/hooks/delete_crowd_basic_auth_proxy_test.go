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

var _ = Describe("User Authn hooks :: delete the crowd-basic-auth-proxy ingress::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")
	Context("There's the crowd-basic-auth-proxy ingress", func() {
		BeforeEach(func() {
			crowdIngress := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: crowd-basic-auth-proxy
  namespace: d8-user-authn
spec:
  ingressClassName: nginx
  rules:
  - host: api.dev.hf.flant.com
    http:
      paths:
      - backend:
          service:
            name: basic-auth-proxy
            port:
              number: 7332
        path: /basic-auth(\/?)(.*)
`
			f.BindingContexts.Set(f.KubeStateSet(crowdIngress))
		})
		It("Should delete the crowd-basic-auth-proxy ingress", func() {
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesResource("Ingress", "d8-user-authn", "crowd-basic-auth-proxy")
			Expect(ingress).To(BeEmpty())
		})
		It("Should not delete the crowd-basic-auth-proxy ingress", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesResource("Ingress", "d8-user-authn", "crowd-basic-auth-proxy")
			Expect(ingress.Field("metadata.name").Str).To(Equal("crowd-basic-auth-proxy"))
		})
	})
	Context("There's the basic-auth-proxy ingress", func() {
		BeforeEach(func() {
			basicIngress := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: basic-auth-proxy
  namespace: d8-user-authn
spec:
  ingressClassName: nginx
  rules:
  - host: api.dev.hf.flant.com
    http:
      paths:
      - backend:
          service:
            name: basic-auth-proxy
            port:
              number: 7332
        path: /basic-auth(\/?)(.*)
`
			f.BindingContexts.Set(f.KubeStateSet(basicIngress))
			f.RunHook()
		})
		It("Should not delete the basic-auth-proxy ingress", func() {
			Expect(f).To(ExecuteSuccessfully())
			ingress := f.KubernetesResource("Ingress", "d8-user-authn", "basic-auth-proxy")
			Expect(ingress.Field("metadata.name").Str).To(Equal("basic-auth-proxy"))
		})
	})
})
