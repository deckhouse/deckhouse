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
	"encoding/base64"
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Node Manager hooks :: generate_webhook_certs ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {"clusterDomain": "`+clusterDomain+`"}},"nodeManager":{"internal":{"capiControllerManagerWebhookCert": {}}}}`, "")

	expectedSANs := []string{
		"capi-webhook-service.d8-cloud-instance-manager",
		"capi-webhook-service.d8-cloud-instance-manager.svc",
		"capi-webhook-service.d8-cloud-instance-manager." + clusterDomain,
		"capi-webhook-service.d8-cloud-instance-manager.svc." + clusterDomain,
	}

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should add ca and certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			caPEM := f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.ca").String()
			crtPEM := f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").String()
			keyPEM := f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.key").String()
			Expect(caPEM).ToNot(BeEmpty())
			Expect(crtPEM).ToNot(BeEmpty())
			Expect(keyPEM).ToNot(BeEmpty())

			tls_certificate.AssertCertBundleValid(GinkgoT(), caPEM, crtPEM, keyPEM, cn, expectedSANs)
		})

		// Direct in-test reproduction of the original bug report:
		//
		//   kubectl -n d8-cloud-instance-manager get secret capi-webhook-tls \
		//     -o jsonpath='{.data.ca\.crt}' | base64 -d > /tmp/capi-ca.crt
		//   kubectl -n d8-cloud-instance-manager get secret capi-webhook-tls \
		//     -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/capi-tls.crt
		//   openssl verify -CAfile /tmp/capi-ca.crt /tmp/capi-tls.crt
		//   # error 18 at 0 depth lookup: self-signed certificate
		//
		// crypto/x509 silently passed the legacy collision (so kube-apiserver
		// was happy), openssl rejected with error 18.
		// AssertOpensslVerifyOK reproduces both checks; see its godoc.
		It("issued bundle must pass `openssl verify -CAfile ca.crt tls.crt`", func() {
			Expect(f).To(ExecuteSuccessfully())

			ca, leaf := tls_certificate.AssertOpensslVerifyOK(
				GinkgoT(),
				f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.ca").String(),
				f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").String(),
			)

			Expect(ca.Subject.String()).To(ContainSubstring("O=Deckhouse"))
			Expect(leaf.Subject.String()).ToNot(ContainSubstring("O=Deckhouse"))
		})
	})
	Context("With secrets", func() {
		caAuthority, _ := genWebhookCa(nil)
		// CN must match the central hook's configured CN, otherwise CN drift
		// triggers a re-issue and the test cert in the secret is replaced.
		tlsAuthority, _ := genWebhookTLS(&go_hook.HookInput{Logger: log.NewNop()}, caAuthority, cn, "capi-webhook-service")

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

			tls_certificate.AssertCertBundleValid(
				GinkgoT(),
				f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.ca").String(),
				f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.crt").String(),
				f.ValuesGet("nodeManager.internal.capiControllerManagerWebhookCert.key").String(),
				cn,
				expectedSANs,
			)
		})
	})
})

func genWebhookCa(logEntry *log.Logger) (*certificate.Authority, error) {
	ca, err := certificate.GenerateCA(logEntry, cn,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"),
		// Match the central hook contract: O=Deckhouse, OU=<module>.
		certificate.WithNames(csr.Name{O: "Deckhouse", OU: "cloud-instance-manager"}),
	)
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
		certificate.WithSigningDefaultUsage([]string{
			"signing",
			"key encipherment",
			"server auth",
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
