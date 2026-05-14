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

const (
	validatingWebhookWithInjectAnnotation = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-vwc
  annotations:
    cert-manager.io/inject-ca-from: d8-cert-manager/test-cert
webhooks:
  - name: test-vwc.deckhouse.io
    sideEffects: None
    admissionReviewVersions: ["v1"]
    clientConfig:
      service:
        namespace: d8-cert-manager
        name: test
        path: /validate
`
	apiServiceWithInjectAnnotation = `
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1alpha1.test.deckhouse.io
  annotations:
    cert-manager.io/inject-apiserver-ca: "true"
spec:
  group: test.deckhouse.io
  version: v1alpha1
  groupPriorityMinimum: 1000
  versionPriority: 15
  service:
    namespace: d8-test
    name: test-api
`
	validatingWebhookWithoutInjectAnnotation = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-vwc
  annotations:
    test.deckhouse.io/some-annotation: "true"
webhooks:
  - name: test-vwc.deckhouse.io
    sideEffects: None
    admissionReviewVersions: ["v1"]
    clientConfig:
      service:
        namespace: d8-cert-manager
        name: test
        path: /validate
`
)

var _ = Describe("Cert Manager hooks :: discover cainjector usage ::", func() {
	f := HookExecutionConfigInit(`{"certManager":{"internal": {}}}`, `{}`)

	Context("Cluster does not have resources that use cainjector", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should disable cainjector", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("Cluster has resource without cainjector annotations", func() {
		BeforeEach(func() {
			f.KubeStateSet(validatingWebhookWithoutInjectAnnotation)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should keep cainjector disabled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeFalse())
		})
	})

	Context("Cluster has ValidatingWebhookConfiguration with inject annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet(validatingWebhookWithInjectAnnotation)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should enable cainjector", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("Cluster has APIService with inject-apiserver-ca annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet(apiServiceWithInjectAnnotation)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should enable cainjector", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("Manual enablement is set in config values", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.ConfigValuesSet("certManager.enableCAInjector", true)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should keep cainjector enabled", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeTrue())
		})
	})

	Context("Resource with annotation is removed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(validatingWebhookWithInjectAnnotation))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeTrue())

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should disable cainjector after resource removal", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(cainjectorEnabledValuesPath).Bool()).To(BeFalse())
		})
	})
})
