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
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: order_tls_auth_certificate", func() {
	f := HookExecutionConfigInit(`{"global":{},"moduleName":{"internal":{}}}`, `{}`)

	selfSignedCA, _ := certificate.GenerateCA(log.NewNop(), "kubernetes")
	tlsAuthCert, _ := certificate.GenerateSelfSignedCert(log.NewNop(),
		"d8-module-name:module-name:auth",
		selfSignedCA,
		certificate.WithGroups("prometheus:auth"),
	)
	incorrectCert, _ := certificate.GenerateSelfSignedCert(log.NewNop(), "test", selfSignedCA)

	Context("Cluster without certificate", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate approved CSR and exit with error", func() {
			tlsAuthCSR, err := dependency.TestDC.K8sClient.CertificatesV1().
				CertificateSigningRequests().
				Get(context.TODO(), "d8-module-name:module-name:auth", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(tlsAuthCSR.Status.Conditions[0].Type).To(Equal(certificatesv1.CertificateApproved))
			Expect(tlsAuthCSR.Spec.Usages).To(Equal([]certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageClientAuth,
			}))
			Expect(tlsAuthCSR.Spec.SignerName).To(Equal(certificatesv1.KubeAPIServerClientSignerName))

			Expect(f).ToNot(ExecuteSuccessfully())
		})
	})

	Context("Cluster with correct certificate", func() {
		BeforeEach(func() {
			tlsAuthSecret := fmt.Sprintf(`
---
apiVersion: v1
data:
  tls.crt: %s
  tls.key: %s
kind: Secret
metadata:
  name: module-name-auth-tls
  namespace: d8-module-name
type: Opaque
`, base64.StdEncoding.EncodeToString([]byte(tlsAuthCert.Cert)), base64.StdEncoding.EncodeToString([]byte(tlsAuthCert.Key)))
			f.BindingContexts.Set(f.KubeStateSet(tlsAuthSecret))
			f.RunHook()
		})

		It("Should persist certs and keys", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("moduleName.internal.moduleAuthTLS.certificate_updated").Exists()).To(BeFalse())
			Expect(f.ValuesGet("moduleName.internal.moduleAuthTLS.key").Exists()).To(BeTrue())

			certFromValues := f.ValuesGet("moduleName.internal.moduleAuthTLS.certificate").String()
			parsedCert, err := helpers.ParseCertificatePEM([]byte(certFromValues))
			if err != nil {
				fmt.Printf("certificate parsing error: %v", err)
			}
			Expect(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Equal(parsedCert.NotBefore)).To(BeFalse())
			Expect(time.Now().Before(parsedCert.NotAfter.AddDate(0, 0, -10))).To(BeTrue())
		})
	})

	Context("Cluster with incorrect certificate", func() {
		BeforeEach(func() {
			tlsAuthSecret := fmt.Sprintf(`
---
apiVersion: v1
data:
  tls.crt: %s
  tls.key: %s
kind: Secret
metadata:
  name: module-name-auth-tls
  namespace: d8-module-name
type: Opaque
`, base64.StdEncoding.EncodeToString([]byte(incorrectCert.Cert)), base64.StdEncoding.EncodeToString([]byte(tlsAuthCert.Key)))
			f.BindingContexts.Set(f.KubeStateSet(tlsAuthSecret))
			f.RunHook()
		})

		It("Should exit with an error (issue new certificate)", func() {
			csr, err := dependency.TestDC.K8sClient.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "d8-module-name:module-name:auth", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(csr.Status.Conditions[0].Type).To(Equal(certificatesv1.CertificateApproved))

			Expect(f).ToNot(ExecuteSuccessfully())
		})
	})
})
