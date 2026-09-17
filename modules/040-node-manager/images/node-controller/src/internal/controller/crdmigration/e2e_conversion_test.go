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

package crdmigration

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const nodeGroupCRDName = "nodegroups.deckhouse.io"

// newCAPEM issues a self-signed CA certificate. The apiserver parses spec.conversion caBundle and
// rejects anything that is not a real PEM certificate, so the specs cannot use a placeholder.
func newCAPEM(commonName string) string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	out := &bytes.Buffer{}
	Expect(pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: der})).To(Succeed())
	return out.String()
}

// newReconciler builds the reconciler the way register.setupController does, with the live reader
// standing in for mgr.GetAPIReader().
func newReconciler() *Reconciler {
	r := &Reconciler{}
	r.InjectClient(k8sClient)
	r.apiReader = k8sClient
	return r
}

func reconcileCRD(crdName string) ctrl.Result {
	res, err := newReconciler().Reconcile(suiteCtx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: crdName},
	})
	Expect(err).NotTo(HaveOccurred())
	return res
}

func createTLSSecret(name, ca string) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: capiNamespace},
		Data:       map[string][]byte{"ca.crt": []byte(ca)},
	}
	Expect(k8sClient.Create(suiteCtx, secret)).To(Succeed())
}

func updateTLSSecret(name, ca string) {
	secret := &corev1.Secret{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name, Namespace: capiNamespace}, secret)).To(Succeed())
	secret.Data["ca.crt"] = []byte(ca)
	Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
}

func getCRD(name string) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, crd)).To(Succeed())
	return crd
}

func conversionWebhookOf(crdName string) *apiextensionsv1.WebhookConversion {
	conversion := getCRD(crdName).Spec.Conversion
	if conversion == nil {
		return nil
	}
	return conversion.Webhook
}

var _ = AfterEach(func() {
	// Only the CRD this suite installs is reset; the other row of the table (instances) is not
	// needed to prove the behaviour and is absent from the environment.
	crd := getCRD(nodeGroupCRDName)
	if crd.Spec.Conversion != nil {
		crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}
		Expect(k8sClient.Update(suiteCtx, crd)).To(Succeed())
	}

	secrets := &corev1.SecretList{}
	Expect(k8sClient.List(suiteCtx, secrets, client.InNamespace(capiNamespace))).To(Succeed())
	for i := range secrets.Items {
		Expect(k8sClient.Delete(suiteCtx, &secrets.Items[i])).To(Succeed())
	}
})

// User story: As a user of NodeGroups, I want the NodeGroup CRD to keep a working conversion
// webhook — including after its CA is rotated — so that reading and writing NodeGroups never
// fails with a conversion error.
var _ = Describe("Conversion webhook reconciliation", func() {
	var testCA, testRotatedCA string

	BeforeEach(func() {
		testCA = newCAPEM("first")
		testRotatedCA = newCAPEM("second")
	})

	It("points the CRD at the node-controller webhook service with the CA from its TLS secret", func() {
		createTLSSecret(nodeControllerWebhookSecretName, testCA)

		reconcileCRD(nodeGroupCRDName)

		webhook := conversionWebhookOf(nodeGroupCRDName)
		Expect(webhook).NotTo(BeNil())
		Expect(getCRD(nodeGroupCRDName).Spec.Conversion.Strategy).To(Equal(apiextensionsv1.WebhookConverter))
		Expect(webhook.ClientConfig.Service.Name).To(Equal(nodeControllerWebhookServiceName))
		Expect(webhook.ClientConfig.Service.Namespace).To(Equal(capiNamespace))
		Expect(*webhook.ClientConfig.Service.Path).To(Equal("/convert"))
		Expect(*webhook.ClientConfig.Service.Port).To(Equal(int32(443)))
		Expect(string(webhook.ClientConfig.CABundle)).To(Equal(testCA))
		Expect(webhook.ConversionReviewVersions).To(Equal([]string{"v1"}))
	})

	It("waits while the node-controller TLS secret is absent", func() {
		res := reconcileCRD(nodeGroupCRDName)

		Expect(res.RequeueAfter).To(BeNumerically(">", 0))
		Expect(conversionWebhookOf(nodeGroupCRDName)).To(BeNil())
	})

	It("waits while the node-controller TLS secret carries an empty ca.crt", func() {
		createTLSSecret(nodeControllerWebhookSecretName, "")

		res := reconcileCRD(nodeGroupCRDName)

		Expect(res.RequeueAfter).To(BeNumerically(">", 0))
		Expect(conversionWebhookOf(nodeGroupCRDName)).To(BeNil())
	})

	It("replaces the caBundle after a CA rotation", func() {
		createTLSSecret(nodeControllerWebhookSecretName, testCA)
		reconcileCRD(nodeGroupCRDName)

		updateTLSSecret(nodeControllerWebhookSecretName, testRotatedCA)
		reconcileCRD(nodeGroupCRDName)

		Expect(string(conversionWebhookOf(nodeGroupCRDName).ClientConfig.CABundle)).To(Equal(testRotatedCA))
	})

	It("does not write the CRD again when the webhook is already current", func() {
		createTLSSecret(nodeControllerWebhookSecretName, testCA)
		reconcileCRD(nodeGroupCRDName)
		afterFirst := getCRD(nodeGroupCRDName).ResourceVersion

		reconcileCRD(nodeGroupCRDName)

		Expect(getCRD(nodeGroupCRDName).ResourceVersion).To(Equal(afterFirst))
	})
})
