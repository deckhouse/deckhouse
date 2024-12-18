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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const clusterDomain = "cluster.local"

var _ = Describe("Node Manager hooks :: generate_webhook_certs ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {"clusterDomain": "`+clusterDomain+`"}},"nodeManager":{"internal":{"capsControllerManagerWebhookCert": {}}}}`, "")

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should add ca and certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.crt").Exists()).To(BeTrue())

			blockCA, _ := pem.Decode([]byte(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.ca").String()))
			certCA, err := x509.ParseCertificate(blockCA.Bytes)
			Expect(err).To(BeNil())
			Expect(certCA.IsCA).To(BeTrue())
			Expect(certCA.Subject.CommonName).To(Equal("caps-controller-manager-webhook"))

			block, _ := pem.Decode([]byte(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.crt").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeFalse())
			Expect(cert.Subject.CommonName).To(Equal("caps-controller-manager-webhook"))
		})
	})
	Context("With secrets", func() {
		caAuthority, _ := genWebhookCa(nil)
		tlsAuthority, _ := genWebhookTLS(&go_hook.HookInput{Logger: log.NewNop()}, caAuthority, "caps-manager-webhook", "caps-controller-manager-webhook-service")

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: caps-controller-manager-webhook-tls
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

			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.ca").String()).To(Equal(caAuthority.Cert))
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.key").String()).To(Equal(tlsAuthority.Key))
			Expect(f.ValuesGet("nodeManager.internal.capsControllerManagerWebhookCert.crt").String()).To(Equal(tlsAuthority.Cert))
		})
	})
})
