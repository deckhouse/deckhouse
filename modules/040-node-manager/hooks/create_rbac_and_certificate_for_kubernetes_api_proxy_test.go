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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// The certificate is issued through tls_certificate.IssueCertificate, which creates a
// CertificateSigningRequest and blocks on csrutil.WaitForCertificate until a signer populates
// its status — something the hook test fake cluster does not provide. The generation path is
// therefore exercised by substituting issueKubeAPIProxyCertificate with a deterministic fake,
// which lets us assert the Secret is created when it is absent while still covering the
// checksum-stability branch (an already-valid certificate must be republished as-is and not
// regenerated).
var _ = Describe("Node Manager hooks :: create_rbac_and_certificate_for_kubernetes_api_proxy ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"kubernetesAPIProxyDiscoveryCert":{}}}}`, "")

	Context("With an existing valid discovery certificate secret", func() {
		proxyCert, err := genKubeAPIProxyDiscoveryCert()
		It("Test fixture certificate must be generated", func() {
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: kubernetes-api-proxy-discovery-cert
  namespace: kube-system
data:
  crt: %[1]s
  key: %[2]s
`, base64.StdEncoding.EncodeToString([]byte(proxyCert.Cert)),
				base64.StdEncoding.EncodeToString([]byte(proxyCert.Key)))),
			)
			f.RunHook()
		})

		It("Should republish the existing certificate into values unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.crt").String()).To(Equal(proxyCert.Cert))
			Expect(f.ValuesGet("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.key").String()).To(Equal(proxyCert.Key))

			// The published certificate must be the pre-existing one (stable across reconciles),
			// not a freshly minted one.
			block, _ := pem.Decode([]byte(f.ValuesGet("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.crt").String()))
			Expect(block).ShouldNot(BeNil())
			cert, parseErr := x509.ParseCertificate(block.Bytes)
			Expect(parseErr).To(BeNil())
			Expect(cert.Subject.CommonName).To(Equal("kubernetes-api-proxy"))
		})

		It("Should leave the existing discovery certificate secret untouched", func() {
			Expect(f).To(ExecuteSuccessfully())

			secret := f.KubernetesResource("Secret", "kube-system", "kubernetes-api-proxy-discovery-cert")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("data.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(proxyCert.Cert))))
			Expect(secret.Field("data.key").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(proxyCert.Key))))
		})
	})

	Context("Without a discovery certificate secret", func() {
		proxyCert, err := genKubeAPIProxyDiscoveryCert()
		It("Test fixture certificate must be generated", func() {
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			// Substitute the CSR-based issuer with a deterministic fake so the generation
			// path runs without a signer in the fake cluster.
			issueKubeAPIProxyCertificate = func(_ *go_hook.HookInput, _ dependency.Container, _ tls_certificate.OrderCertificateRequest) (*tls_certificate.CertificateInfo, error) {
				return &tls_certificate.CertificateInfo{Certificate: proxyCert.Cert, Key: proxyCert.Key}, nil
			}
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		})

		AfterEach(func() {
			issueKubeAPIProxyCertificate = tls_certificate.IssueCertificate
		})

		It("Should create the discovery certificate secret that did not exist before the run", func() {
			// The secret is absent before the hook runs.
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-api-proxy-discovery-cert").Exists()).To(BeFalse())

			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())

			// The secret now exists and holds the issued certificate.
			secret := f.KubernetesResource("Secret", "kube-system", "kubernetes-api-proxy-discovery-cert")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("data.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(proxyCert.Cert))))
			Expect(secret.Field("data.key").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(proxyCert.Key))))

			// ... and the same certificate is published into the module values.
			Expect(f.ValuesGet("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.crt").String()).To(Equal(proxyCert.Cert))
			Expect(f.ValuesGet("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.key").String()).To(Equal(proxyCert.Key))
		})
	})
})

// genKubeAPIProxyDiscoveryCert issues a long-lived (10y) client certificate that mimics the
// contents of the kube-system/kubernetes-api-proxy-discovery-cert secret.
func genKubeAPIProxyDiscoveryCert() (*certificate.Certificate, error) {
	ca, err := certificate.GenerateCA(log.NewNop(), "kubernetes-api-proxy",
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	if err != nil {
		return nil, fmt.Errorf("cannot generate CA: %v", err)
	}

	crt, err := certificate.GenerateSelfSignedCert(log.NewNop(),
		"kubernetes-api-proxy",
		ca,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry((24*time.Hour)*365*10),
		certificate.WithSigningDefaultUsage([]string{"signing", "key encipherment", "client auth"}),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot generate certificate: %v", err)
	}

	return &crt, nil
}
