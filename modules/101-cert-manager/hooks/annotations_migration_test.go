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

var _ = Describe("Modules :: cert-manager :: hooks :: annotations_migration ::", func() {

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Ingress with legacy annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    certmanager.k8s.io/cluster-issuer: letsencrypt
    kubernetes.io/tls-acme: "true"
  name: test
  namespace: default
` + testIngressSpec)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Ingress annotations should change", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.PatchCollector.Operations()).To(HaveLen(1))

			ing := f.KubernetesResource("Ingress", "default", "test")
			Expect(ing.Field("metadata.annotations.kubernetes\\.io/tls-acme").Exists()).To(BeTrue())
			Expect(ing.Field("metadata.annotations.certmanager\\.k8s\\.io/cluster-issuer").String()).To(Equal("letsencrypt"))
			Expect(ing.Field("metadata.annotations.cert-manager\\.io/cluster-issuer").String()).To(Equal("letsencrypt"))
		})
	})

	Context("Ingress with legacy annotation already migrated", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cert-manager-migrated
  namespace: d8-cert-manager
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    certmanager.k8s.io/cluster-issuer: letsencrypt
    kubernetes.io/tls-acme: "true"
  name: test
  namespace: default
` + testIngressSpec)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Ingress annotations should not change", func() {
			Expect(f).To(ExecuteSuccessfully())
			ing := f.KubernetesResource("Ingress", "default", "test")
			Expect(ing.Field("metadata.annotations.kubernetes\\.io/tls-acme").Exists()).To(BeTrue())
			Expect(ing.Field("metadata.annotations.certmanager\\.k8s\\.io/cluster-issuer").String()).To(Equal("letsencrypt"))
			Expect(ing.Field("metadata.annotations.cert-manager\\.io/cluster-issuer").Exists()).To(BeFalse())
		})
	})

	Context("Ingress with both annotations", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    cert-manager.io/cluster-issuer: not-letsencrypt
    certmanager.k8s.io/cluster-issuer: letsencrypt
    kubernetes.io/tls-acme: "true"
  name: test
  namespace: default
` + testIngressSpec)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Ingress annotations should not change", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Check that hook did not generate excessive patches for object that should not be mutated
			Expect(f.PatchCollector.Operations()).To(HaveLen(0))

			ing := f.KubernetesResource("Ingress", "default", "test")
			Expect(ing.Field("metadata.annotations.kubernetes\\.io/tls-acme").Exists()).To(BeTrue())
			Expect(ing.Field("metadata.annotations.certmanager\\.k8s\\.io/cluster-issuer").String()).To(Equal("letsencrypt"))
			Expect(ing.Field("metadata.annotations.cert-manager\\.io/cluster-issuer").String()).To(Equal("not-letsencrypt"))
		})
	})
})

const testIngressSpec = `
spec:
  rules:
  - host: test.ru
    http:
      paths:
      - backend:
          service:
            name: test
            port:
              number: 8080
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - test.ru
    secretName: test-tls
`
