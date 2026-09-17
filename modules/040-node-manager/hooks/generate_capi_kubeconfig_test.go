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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedcertificatesv1 "k8s.io/client-go/kubernetes/typed/certificates/v1"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	capiKubeconfigTestCSRName    = "capi-controller-manager"
	capiKubeconfigTestSecretName = "static-kubeconfig"
	capiKubeconfigSignerTimeout  = 5 * time.Second
	capiKubeconfigSignerPoll     = 20 * time.Millisecond
	capiKubeconfigFreshCertLeft  = 100 * 24 * time.Hour
	capiKubeconfigStaleCertLeft  = 5 * 24 * time.Hour
	capiKubeconfigForeignHash    = "0000000000000000000000000000000000000000000000000000000000000000"
)

var _ = Describe("Node Manager hooks :: generate_capi_kubeconfig ::", func() {
	f := HookExecutionConfigInit(`{"global":{},"nodeManager":{"internal":{"capsControllerManagerEnabled":true}}}`, "")

	selfSignedCA, err := certificate.GenerateCA(log.NewNop(), "kubernetes")
	if err != nil {
		panic(err)
	}
	signedCert, err := certificate.GenerateSelfSignedCert(log.NewNop(), capiKubeconfigTestCSRName, selfSignedCA)
	if err != nil {
		panic(err)
	}

	csrClient := func() typedcertificatesv1.CertificateSigningRequestInterface {
		return dependency.TestDC.K8sClient.CertificatesV1().CertificateSigningRequests()
	}

	currentEndpointHash := func() string {
		restConfig, err := dependency.TestDC.GetClientConfig()
		Expect(err).ToNot(HaveOccurred())
		return apiserverEndpointHash(restConfig.Host, restConfig.CAData)
	}

	kubeconfigSecret := func(certLeft time.Duration, endpointHash string) string {
		return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
type: cluster.x-k8s.io/secret
metadata:
  name: %s
  namespace: d8-cloud-instance-manager
  labels:
    cluster.x-k8s.io/cluster-name: static
  annotations:
    node-manager.deckhouse.io/certificate-expires-at: "%s"
    node-manager.deckhouse.io/endpoint-hash: "%s"
data:
  value: a3ViZWNvbmZpZw==
`, capiKubeconfigTestSecretName, time.Now().Add(certLeft).UTC().Format(time.RFC3339), endpointHash)
	}

	// The fake cluster has no signer: once the hook approves its CSR, hand it a certificate
	// so IssueCertificate returns instead of waiting a minute for nothing.
	signApprovedCSR := func() {
		go func() {
			deadline := time.Now().Add(capiKubeconfigSignerTimeout)
			for time.Now().Before(deadline) {
				time.Sleep(capiKubeconfigSignerPoll)
				csr, err := csrClient().Get(context.TODO(), capiKubeconfigTestCSRName, metav1.GetOptions{})
				if err != nil || !csrApproved(csr) {
					continue
				}
				csr.Status.Certificate = []byte(signedCert.Cert)
				_, _ = csrClient().UpdateStatus(context.TODO(), csr, metav1.UpdateOptions{})
				return
			}
		}()
	}

	runWithState := func(state string) {
		_ = csrClient().Delete(context.TODO(), capiKubeconfigTestCSRName, metav1.DeleteOptions{})
		f.BindingContexts.Set(f.KubeStateSet(state), f.GenerateBeforeHelmContext())
		f.RunHook()
	}

	expectNewKubeconfig := func() {
		Expect(f).To(ExecuteSuccessfully())

		_, err := csrClient().Get(context.TODO(), capiKubeconfigTestCSRName, metav1.GetOptions{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the issued CSR must be deleted")

		secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", capiKubeconfigTestSecretName)
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field(`data.value`).String()).ToNot(Equal("a3ViZWNvbmZpZw=="), "kubeconfig must be rewritten")
		Expect(secret.Field(`metadata.annotations.node-manager\.deckhouse\.io/endpoint-hash`).String()).To(Equal(currentEndpointHash()))
		expiresAt, err := time.Parse(time.RFC3339, secret.Field(`metadata.annotations.node-manager\.deckhouse\.io/certificate-expires-at`).String())
		Expect(err).ToNot(HaveOccurred())
		Expect(expiresAt.After(time.Now().Add(capiKubeconfigRenewMargin))).To(BeTrue())
	}

	Context("Secret with a long living certificate for the current endpoint", func() {
		BeforeEach(func() {
			runWithState(kubeconfigSecret(capiKubeconfigFreshCertLeft, currentEndpointHash()))
		})

		It("Should reuse the kubeconfig and issue nothing", func() {
			Expect(f).To(ExecuteSuccessfully())

			_, err := csrClient().Get(context.TODO(), capiKubeconfigTestCSRName, metav1.GetOptions{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "no CSR must be created")

			Expect(f.PatchCollector.Operations()).To(BeEmpty())
		})
	})

	Context("Secret with a certificate inside the renew margin", func() {
		BeforeEach(func() {
			signApprovedCSR()
			runWithState(kubeconfigSecret(capiKubeconfigStaleCertLeft, currentEndpointHash()))
		})

		It("Should issue a new certificate and annotate the secret", expectNewKubeconfig)
	})

	Context("Secret issued for another apiserver endpoint", func() {
		BeforeEach(func() {
			signApprovedCSR()
			runWithState(kubeconfigSecret(capiKubeconfigFreshCertLeft, capiKubeconfigForeignHash))
		})

		It("Should issue a new certificate and annotate the secret", expectNewKubeconfig)
	})

	Context("Without a kubeconfig secret", func() {
		BeforeEach(func() {
			signApprovedCSR()
			runWithState(``)
		})

		It("Should issue a new certificate and annotate the secret", expectNewKubeconfig)
	})
})

func csrApproved(csr *certificatesv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved {
			return true
		}
	}
	return false
}
