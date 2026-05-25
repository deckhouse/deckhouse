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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Node Manager hooks :: generate_webhook_certs ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {"clusterDomain": "`+clusterDomain+`"}},"nodeManager":{"internal":{"capiControllerManagerWebhookCert": {}}}}`, "")

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should add ca and certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").Exists()).To(BeTrue())

			blockCA, _ := pem.Decode([]byte(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.ca").String()))
			certCA, err := x509.ParseCertificate(blockCA.Bytes)
			Expect(err).To(BeNil())
			Expect(certCA.IsCA).To(BeTrue())
			Expect(certCA.Subject.CommonName).To(Equal("capi-controller-manager-webhook"))

			block, _ := pem.Decode([]byte(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeFalse())
			Expect(cert.Subject.CommonName).To(Equal("capi-controller-manager-webhook"))
		})
	})
	Context("With secrets", func() {
		caAuthority, _ := genWebhookCa(nil)
		tlsAuthority, _ := genWebhookTLS(&go_hook.HookInput{Logger: log.NewNop()}, caAuthority, "capi-manager-webhook", "capi-webhook-service")

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: capi-webhook-tls
  namespace: d8-cloud-instance-manager
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

			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.ca").String()).To(Equal(caAuthority.Cert))
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.key").String()).To(Equal(tlsAuthority.Key))
			Expect(f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").String()).To(Equal(tlsAuthority.Cert))
		})
	})
})

func genWebhookCa(logEntry *log.Logger) (*certificate.Authority, error) {
	ca, err := certificate.GenerateCA(logEntry, cn, certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	if err != nil {
		return nil, fmt.Errorf("cannot generate CA: %v", err)
	}

	return &ca, nil
}

func genWebhookTLS(input *go_hook.HookInput, ca *certificate.Authority, cn string, sanPrefix string) (*certificate.Certificate, error) {
	tls, err := certificate.GenerateSelfSignedCert(input.Logger,
		cn,
		*ca,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry((24*time.Hour)*365*10),
		certificate.WithSigningDefaultUsage([]string{"signing",
			"key encipherment",
			"requestheader-client",
		}),
		certificate.WithSANs(
			sanPrefix+".d8-cloud-instance-manager",
			sanPrefix+".d8-cloud-instance-manager.svc",
			sanPrefix+".d8-cloud-instance-manager."+clusterDomain,
			sanPrefix+".d8-cloud-instance-manager.svc."+clusterDomain,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot generate TLS: %v", err)
	}

	return &tls, err
}
