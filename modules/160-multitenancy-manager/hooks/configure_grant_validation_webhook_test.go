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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: multitenancy-manager :: hooks :: configure_grant_validation_webhook ::", func() {
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

	staleWebhook := func() *admissionregistrationv1.ValidatingWebhookConfiguration {
		return &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: validatingWebhookConfigurationName},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:                    "cluster-objects-grants-validator.multitenancy.deckhouse.io",
					AdmissionReviewVersions: []string{"v1"},
					SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						CABundle: []byte("old-ca"),
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "multitenancy-manager",
							Namespace: "d8-multitenancy-manager",
							Path:      ptr.To("/is-granted"),
							Port:      ptr.To(int32(9443)),
						},
					},
				},
			},
		}
	}

	seedStaleWebhook := func(f *HookExecutionConfig) {
		_, err := f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(
			context.TODO(), staleWebhook(), metav1.CreateOptions{},
		)
		Expect(err).ShouldNot(HaveOccurred())
	}

	readWebhook := func(f *HookExecutionConfig) *admissionregistrationv1.ValidatingWebhookConfiguration {
		vwc, err := f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
			context.TODO(), validatingWebhookConfigurationName, metav1.GetOptions{},
		)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(vwc.Webhooks).ToNot(BeEmpty())
		return vwc
	}

	f := HookExecutionConfigInit(initValues, `{}`)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceDefinition", false)
	f.RegisterCRD("multitenancy.deckhouse.io", "v1alpha1", "GrantableClusterResourceReference", false)

	Context("existing webhook with a stale CA and a rotated Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState + secretNewCA))
			seedStaleWebhook(f)
			f.RunHook()
		})

		It("replaces caBundle with ca.crt from the Secret on the update path", func() {
			Expect(f).To(ExecuteSuccessfully())
			vwc := readWebhook(f)
			Expect(string(vwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("new-ca"))
			Expect(vwc.Webhooks[0].Rules).ToNot(BeEmpty())
			Expect(vwc.Webhooks[0].FailurePolicy).ToNot(BeNil())
			Expect(*vwc.Webhooks[0].FailurePolicy).To(Equal(admissionregistrationv1.Fail))
		})
	})

	Context("existing webhook, no Secret, CA only in values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState))
			seedStaleWebhook(f)
			f.RunHook()
		})

		It("writes the CA from module values on the update path", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(readWebhook(f).Webhooks[0].ClientConfig.CABundle)).To(Equal("values-ca"))
		})
	})

	Context("webhook is missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState + secretNewCA))
			f.RunHook()
		})

		It("creates the configuration with the current CA", func() {
			Expect(f).To(ExecuteSuccessfully())
			vwc := readWebhook(f)
			Expect(string(vwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("new-ca"))
			Expect(vwc.Webhooks[0].ClientConfig.Service).NotTo(BeNil())
			Expect(vwc.Webhooks[0].ClientConfig.Service.Path).NotTo(BeNil())
			Expect(*vwc.Webhooks[0].ClientConfig.Service.Path).To(Equal("/is-granted"))
		})
	})

	Context("certificate is not issued yet and the webhook is missing", func() {
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

	Context("certificate is not issued yet but the webhook already exists", func() {
		BeforeEach(func() {
			f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "")
			f.BindingContexts.Set(f.KubeStateSet(grantableState))
			seedStaleWebhook(f)
			f.RunHook()
		})

		It("keeps the existing CA and still reconciles rules", func() {
			Expect(f).To(ExecuteSuccessfully())
			vwc := readWebhook(f)
			Expect(string(vwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("old-ca"))
			Expect(vwc.Webhooks[0].Rules).ToNot(BeEmpty())
		})
	})

	Context("existing webhook with an empty webhooks list", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(grantableState + secretNewCA))
			_, err := f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(
				context.TODO(),
				&admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: validatingWebhookConfigurationName},
				},
				metav1.CreateOptions{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			f.RunHook()
		})

		It("rebuilds webhooks[0] instead of failing the queue", func() {
			Expect(f).To(ExecuteSuccessfully())
			vwc := readWebhook(f)
			Expect(string(vwc.Webhooks[0].ClientConfig.CABundle)).To(Equal("new-ca"))
			Expect(vwc.Webhooks[0].Rules).ToNot(BeEmpty())
		})
	})

	Context("existing webhook with an empty webhooks list and no CA", func() {
		BeforeEach(func() {
			f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "")
			f.BindingContexts.Set(f.KubeStateSet(grantableState))
			_, err := f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(
				context.TODO(),
				&admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: validatingWebhookConfigurationName},
				},
				metav1.CreateOptions{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			f.RunHook()
		})

		It("fails instead of publishing a Fail webhook without a CA", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError(errWebhookCertNotIssued))
			vwc, err := f.KubeClient().AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				context.TODO(), validatingWebhookConfigurationName, metav1.GetOptions{},
			)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(vwc.Webhooks).To(BeEmpty())
		})
	})
})
