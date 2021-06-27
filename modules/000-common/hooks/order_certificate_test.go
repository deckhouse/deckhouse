/*
Copyright 2021 Flant CJSC

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
	"os"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: order_certificate_test", func() {
	f := HookExecutionConfigInit(`{"global":{},"moduleName":{"internal":{}}}`, `{}`)

	var log = logrus.New()
	log.Level = logrus.InfoLevel
	log.Out = os.Stdout
	var logEntry = log.WithContext(context.TODO())

	selfSignedCA, _ := certificate.GenerateCA(logEntry, "kubernetes")
	cert, _ := certificate.GenerateSelfSignedCert(logEntry, "test", []string{"test.kube-system.svc"}, selfSignedCA)

	Context("Cluster without certificate", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate approved CSR and exit with error", func() {
			csr, err := dependency.TestDC.K8sClient.CertificatesV1beta1().CertificateSigningRequests().Get(context.TODO(), "d8-module-name:module-name:auth", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(csr.Status.Conditions[0].Type).To(Equal(certificatesv1beta1.CertificateApproved))

			Expect(f).ToNot(ExecuteSuccessfully())
		})
	})

	Context("Cluster with certificate", func() {
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
`, base64.StdEncoding.EncodeToString([]byte(cert.Cert)), base64.StdEncoding.EncodeToString([]byte(cert.Key)))
			tlsAuthSecret2 := fmt.Sprintf(`
---
apiVersion: v1
data:
  tls.crt: %s
  tls.key: %s
kind: Secret
metadata:
  name: module-name-access-tls
  namespace: d8-module-name
type: Opaque
`, base64.StdEncoding.EncodeToString([]byte(cert.Cert)), base64.StdEncoding.EncodeToString([]byte(cert.Key)))

			f.BindingContexts.Set(f.KubeStateSet(tlsAuthSecret + tlsAuthSecret2))
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

			Expect(f.ValuesGet("moduleName.internal.moduleAccessTLS.certificate_updated").Exists()).To(BeFalse())
			Expect(f.ValuesGet("moduleName.internal.moduleAccessTLS.key").Exists()).To(BeTrue())

			cert2FromValues := f.ValuesGet("moduleName.internal.moduleAccessTLS.certificate").String()
			parsedCert2, err := helpers.ParseCertificatePEM([]byte(cert2FromValues))
			if err != nil {
				fmt.Printf("certificate parsing error: %v", err)
			}
			Expect(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Equal(parsedCert2.NotBefore)).To(BeFalse())
			Expect(time.Now().Before(parsedCert2.NotAfter.AddDate(0, 0, -10))).To(BeTrue())
		})
	})

})
