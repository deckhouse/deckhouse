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

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Cert Manager hooks :: generate_yandex_webhook_certs ::", func() {
	Context("Yandex DNS disabled", func() {
		f := HookExecutionConfigInit(`{"certManager":{"internal":{}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should skip generating yandex webhook certificates", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.ca").Exists()).To(BeFalse())
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.crt").Exists()).To(BeFalse())
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.key").Exists()).To(BeFalse())
		})
	})

	Context("Yandex DNS enabled without secret", func() {
		f := HookExecutionConfigInit(`{"certManager":{"yandexFolderID":"b1gabcdefghijklmnopq","yandexServiceAccountJSON":"eyJpZCI6ImFqZSJ9","internal":{"yandexWebhookCert":{}}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should add ca and certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.ca").Exists()).To(BeTrue())

			blockCA, _ := pem.Decode([]byte(f.ValuesGet("certManager.internal.yandexWebhookCert.ca").String()))
			certCA, err := x509.ParseCertificate(blockCA.Bytes)
			Expect(err).To(BeNil())
			Expect(certCA.IsCA).To(BeTrue())
			Expect(certCA.Subject.CommonName).To(Equal("yandex-dns-webhook"))

			block, _ := pem.Decode([]byte(f.ValuesGet("certManager.internal.yandexWebhookCert.crt").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeFalse())
			Expect(cert.Subject.CommonName).To(Equal("yandex-dns-webhook"))
		})
	})

	Context("Yandex DNS enabled with secret", func() {
		f := HookExecutionConfigInit(`{"certManager":{"yandexFolderID":"b1gabcdefghijklmnopq","yandexServiceAccountJSON":"eyJpZCI6ImFqZSJ9","internal":{"yandexWebhookCert":{}}}}`, "")
		caAuthority, _ := genYandexWebhookCa(nil)
		tlsAuthority, _ := genYandexWebhookTLS(&go_hook.HookInput{Logger: log.NewNop()}, caAuthority)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: yandex-dns-webhook-tls
  namespace: d8-cert-manager
data:
  ca.crt: %[1]s
  tls.crt: %[2]s
  tls.key: %[3]s
`, base64.StdEncoding.EncodeToString([]byte(caAuthority.Cert)),
				base64.StdEncoding.EncodeToString([]byte(tlsAuthority.Cert)),
				base64.StdEncoding.EncodeToString([]byte(tlsAuthority.Key)))),
			)
			f.RunHook()
		})

		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.ca").String()).To(Equal(caAuthority.Cert))
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.key").String()).To(Equal(tlsAuthority.Key))
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.crt").String()).To(Equal(tlsAuthority.Cert))
		})
	})

	Context("Only one Yandex setting is set", func() {
		f := HookExecutionConfigInit(`{"certManager":{"yandexFolderID":"b1gabcdefghijklmnopq","internal":{}}}`, "")

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should skip generating yandex webhook certificates", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("certManager.internal.yandexWebhookCert.ca").Exists()).To(BeFalse())
		})
	})
})

func genYandexWebhookCa(logEntry *log.Logger) (*certificate.Authority, error) {
	const cn = "yandex-dns-webhook"
	ca, err := certificate.GenerateCA(logEntry, cn, func(r *csr.CertificateRequest) {
		r.KeyRequest = &csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}
		r.Hosts = []string{
			"yandex-dns-webhook.d8-cert-manager.svc",
			"yandex-dns-webhook.d8-cert-manager",
			"yandex-dns-webhook",
		}
		r.Names = []csr.Name{
			{O: "yandex-dns-webhook.d8-cert-manager"},
		}
	})
	if err != nil {
		return nil, fmt.Errorf("cannot generate CA: %v", err)
	}

	return &ca, nil
}

func genYandexWebhookTLS(input *go_hook.HookInput, ca *certificate.Authority) (*certificate.Certificate, error) {
	tls, err := certificate.GenerateSelfSignedCert(input.Logger,
		"yandex-dns-webhook",
		*ca,
		certificate.WithGroups(
			"yandex-dns-webhook.d8-cert-manager",
		),
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithSANs(
			"yandex-dns-webhook.d8-cert-manager.svc",
			"yandex-dns-webhook.d8-cert-manager",
			"yandex-dns-webhook",
		),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot generate TLS: %v", err)
	}

	return &tls, err
}
