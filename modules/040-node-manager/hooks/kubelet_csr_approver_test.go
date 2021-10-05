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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	cv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var (
	csrTemplate = `
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  creationTimestamp: null
  generateName: csr-
  name: csr-96llc
spec:
  groups:
  - system:nodes
  - system:authenticated
  request: %s
  signerName: kubernetes.io/kubelet-serving
  usages:
  - digital signature
  - key encipherment
  - server auth
  username: system:node:dev-master-0
`
	csr1 *cv1.CertificateSigningRequest
)

var _ = Describe("Modules :: nodeManager :: hooks :: kubelet_csr_approver ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with proper csr", func() {
		BeforeEach(func() {
			var (
				buf       bytes.Buffer
				csrBytes  []byte
				base64Csr string
			)

			keyBytes, _ := rsa.GenerateKey(rand.Reader, 1024)
			x509cr := x509.CertificateRequest{
				Subject: pkix.Name{
					Organization: []string{"system:nodes"},
					CommonName:   "system:node:dev-master-0",
				},
				DNSNames:    []string{"system:nodes"},
				IPAddresses: []net.IP{net.ParseIP("1.2.3.4")},
			}
			csrBytes, _ = x509.CreateCertificateRequest(rand.Reader, &x509cr, keyBytes)
			_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
			base64Csr = base64.StdEncoding.EncodeToString(buf.Bytes())
			csrRequiredApproval := fmt.Sprintf(csrTemplate, base64Csr)

			_ = yaml.Unmarshal([]byte(csrRequiredApproval), &csr1)

			f.BindingContexts.Set(f.KubeStateSet(csrRequiredApproval))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr1, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "csr-96llc", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions[0].Type).To(Equal(cv1.CertificateApproved))
		})
	})

	Context("Cluster with wrong csr (Organization and DNSNames must match 'system:nodes')", func() {
		BeforeEach(func() {
			var (
				buf       bytes.Buffer
				csrBytes  []byte
				base64Csr string
			)

			keyBytes, _ := rsa.GenerateKey(rand.Reader, 1024)
			x509cr := x509.CertificateRequest{
				Subject: pkix.Name{
					Organization: []string{"foobar"},
					CommonName:   "system:node:dev-master-0",
				},
				DNSNames:    []string{"foobar"},
				IPAddresses: []net.IP{net.ParseIP("1.2.3.4")},
			}
			csrBytes, _ = x509.CreateCertificateRequest(rand.Reader, &x509cr, keyBytes)
			_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
			base64Csr = base64.StdEncoding.EncodeToString(buf.Bytes())
			csrRequiredApproval := fmt.Sprintf(csrTemplate, base64Csr)

			_ = yaml.Unmarshal([]byte(csrRequiredApproval), &csr1)

			f.BindingContexts.Set(f.KubeStateSet(csrRequiredApproval))
			_, _ = f.KubeClient().CertificatesV1().CertificateSigningRequests().Create(context.TODO(), csr1, metav1.CreateOptions{})

			f.RunHook()
		})

		It("Must be executed successfully and don't approve csr", func() {
			Expect(f).To(ExecuteSuccessfully())
			csr, err := f.KubeClient().CertificatesV1().CertificateSigningRequests().Get(context.TODO(), "csr-96llc", metav1.GetOptions{})
			Expect(err).To(BeNil())
			Expect(csr.Status.Conditions).To(BeNil())
		})
	})

})
