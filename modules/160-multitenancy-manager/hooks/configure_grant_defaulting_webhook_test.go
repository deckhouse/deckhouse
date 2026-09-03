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
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: configure_grant_defaulting_webhook ::", func() {
	const initValues = `
global:
  discovery: {}
multitenancyManager:
  internal:
    admissionWebhookCert:
      ca: values-ca
      crt: dummy
      key: dummy
`

	const grantableState = `
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: clusterroles
spec:
  enforcement: Managed
  defaultAvailability: All
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: clusterroles-rolebinding
spec:
  grantableClusterResourceName: clusterroles
  rule:
    apiGroups: ["rbac.authorization.k8s.io"]
    apiVersions: ["v1"]
    resources: ["rolebindings"]
`

	const secretNewCA = `
---
apiVersion: v1
kind: Secret
metadata:
  name: admission-webhook-certs
  namespace: d8-multitenancy-manager
type: kubernetes.io/tls
data:
  ca.crt: bmV3LWNh
  tls.crt: ZHVtbXk=
  tls.key: ZHVtbXk=
`

	const staleMutatingWebhook = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: cluster-objects-grants-defaulting
webhooks:
  - name: cluster-objects-grants-defaulting.multitenancy.deckhouse.io
    admissionReviewVersions: ["v1"]
    sideEffects: None
    clientConfig:
      caBundle: b2xkLWNh
      service:
        name: multitenancy-manager
        namespace: d8-multitenancy-manager
        path: /defaults
        port: 9443
    rules:
      - apiGroups: ["rbac.authorization.k8s.io"]
        apiVersions: ["*"]
        resources: ["rolebindings"]
        operations: ["CREATE", "UPDATE"]
`

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceReference", false)

	Context("existing webhook with a stale CA and a rotated Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState + secretNewCA + staleMutatingWebhook))
			f.RunHook()
		})

		It("replaces caBundle with ca.crt from the Secret, not the create-time value", func() {
			Expect(f).To(ExecuteSuccessfully())
			mwc, err := f.KubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
				context.TODO(), mutatingWebhookConfigurationName, metav1.GetOptions{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(mwc.Webhooks).ToNot(BeEmpty())
			Expect(string(mwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("new-ca"))
			Expect(mwc.Webhooks[0].Rules).ToNot(BeEmpty())
		})
	})

	Context("existing webhook, no Secret, CA only in values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState + staleMutatingWebhook))
			f.RunHook()
		})

		It("writes the CA from module values", func() {
			Expect(f).To(ExecuteSuccessfully())
			mwc, err := f.KubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
				context.TODO(), mutatingWebhookConfigurationName, metav1.GetOptions{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(mwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("values-ca"))
		})
	})

	Context("webhook is missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState + secretNewCA))
			f.RunHook()
		})

		It("creates the configuration with the current CA", func() {
			Expect(f).To(ExecuteSuccessfully())
			mwc, err := f.KubeClient().AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
				context.TODO(), mutatingWebhookConfigurationName, metav1.GetOptions{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(mwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("new-ca"))
			Expect(mwc.Webhooks[0].ClientConfig.Service).NotTo(BeNil())
			Expect(mwc.Webhooks[0].ClientConfig.Service.Path).NotTo(BeNil())
			Expect(*mwc.Webhooks[0].ClientConfig.Service.Path).To(Equal("/defaults"))
		})
	})

	Context("certificate is not issued yet", func() {
		BeforeEach(func() {
			f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "")
			f.BindingContexts.Set(f.KubeStateSet(grantableState))
			f.RunHook()
		})

		It("fails instead of creating a webhook the apiserver cannot trust", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError(errWebhookCertNotIssued))
		})
	})
})
