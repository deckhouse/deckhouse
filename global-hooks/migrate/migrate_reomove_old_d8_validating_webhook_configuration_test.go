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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	oldValidationWebhookConfigMocks = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: d8-deckhouse-validating-webhook-handler
webhooks:
- admissionReviewVersions:
  - v1
  - v1beta1
  clientConfig:
    caBundle: QUFBCg==
    service:
      name: validating-webhook-handler
      namespace: d8-system
      path: /hooks/cluster-authorization-rules-deckhouse-io
      port: 443
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: cluster-authorization-rules.deckhouse.io
  namespaceSelector: {}
  objectSelector: {}
  rules:
  - apiGroups:
    - deckhouse.io
    apiVersions:
    - '*'
    operations:
    - CREATE
    - UPDATE
    resources:
    - clusterauthorizationrules
    scope: Cluster
  sideEffects: None
  timeoutSeconds: 10
`

	newValidationWebhookConfigMocks = `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: d8-deckhouse-validating-webhook-handler-hooks
webhooks:
- admissionReviewVersions:
  - v1
  - v1beta1
  clientConfig:
    caBundle: QUFBCg==
    service:
      name: validating-webhook-handler
      namespace: d8-system
      path: /hooks/d8-cluster-configuration-secret-deckhouse-io
      port: 443
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: d8-cluster-configuration-secret.deckhouse.io
  namespaceSelector: {}
  objectSelector:
    matchLabels:
      name: d8-cluster-configuration
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - '*'
    resources:
    - secrets
    scope: Namespaced
  sideEffects: None
  timeoutSeconds: 10
`
)

func createValidationWebhookConfigMocks(doc string) {
	var c admissionv1.ValidatingWebhookConfiguration
	err := yaml.Unmarshal([]byte(doc), &c)
	if err != nil {
		panic(err)
	}
	_, err = dependency.TestDC.MustGetK8sClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), &c, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Global :: migrate_reomove_old_d8_validating_webhook_configuration ::", func() {
	getValidationWebhookConfigMocks := func(name string) (*admissionv1.ValidatingWebhookConfiguration, error) {
		return dependency.TestDC.MustGetK8sClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), name, metav1.GetOptions{})
	}

	assertOldConfigurationWasDeleted := func() {
		_, err := getValidationWebhookConfigMocks("d8-deckhouse-validating-webhook-handler")
		Expect(errors.IsNotFound(err)).To(BeTrue())
	}

	assertNewConfigurationKeep := func() {
		c, err := getValidationWebhookConfigMocks("d8-deckhouse-validating-webhook-handler-hooks")
		Expect(err).ToNot(HaveOccurred())
		Expect(c).ToNot(BeNil())
		Expect(c.GetName()).To(Equal("d8-deckhouse-validating-webhook-handler-hooks"))
	}

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.RunHook()
		})

		It("Hook should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Old and new configuration exists", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createValidationWebhookConfigMocks(oldValidationWebhookConfigMocks)
			createValidationWebhookConfigMocks(newValidationWebhookConfigMocks)
			f.RunHook()
		})

		It("Should delete old configuration and keep new", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertOldConfigurationWasDeleted()

			assertNewConfigurationKeep()
		})
	})

	Context("Only new configuration exists", func() {
		f := HookExecutionConfigInit(`{}`, `{}`)

		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			createValidationWebhookConfigMocks(newValidationWebhookConfigMocks)
			f.RunHook()
		})

		It("should not fail and keep new configuration", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertOldConfigurationWasDeleted()

			assertNewConfigurationKeep()
		})
	})
})
