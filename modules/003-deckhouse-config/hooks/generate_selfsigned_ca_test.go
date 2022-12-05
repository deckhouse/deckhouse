/*
Copyright 2022 Flant JSC

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	webhookHandlerCertPath = "deckhouseConfig.internal.webhookCert.crt"
	webhookHandlerKeyPath  = "deckhouseConfig.internal.webhookCert.key"
	webhookHandlerCAPath   = "deckhouseConfig.internal.webhookCert.ca"

	initValuesString = `{
  "deckhouseConfig": {
    "internal": {
      "webhookCert":{}
    }
  }
}`
	initConfigValuesString = `{}`
)

var _ = Describe("DeckhouseConfig hooks :: generate self-signed CA :: ", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("giving no Secret", func() {
		var createdSecret string

		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should generate new certificates and set values", func() {
			Expect(f).To(ExecuteSuccessfully())
			caValue := f.ValuesGet(webhookHandlerCAPath)
			keyValue := f.ValuesGet(webhookHandlerKeyPath)
			certValue := f.ValuesGet(webhookHandlerCertPath)
			Expect(certValue.Exists()).To(BeTrue())
			Expect(keyValue.Exists()).To(BeTrue())
			Expect(caValue.Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet(webhookHandlerCAPath).String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet(webhookHandlerCertPath).String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: fmt.Sprintf("%s.%s.svc", webhookServiceHost, webhookServiceNamespace),
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

			// Save generated certificate for further testing.
			createdSecret = createWebhookSecret(caValue.String(), certValue.String(), keyValue.String())
		})

		Context("giving existing Secret and empty values", func() {
			BeforeEach(func() {
				// Clear values.
				f.ValuesDelete(webhookHandlerCertPath)
				f.ValuesDelete(webhookHandlerKeyPath)
				f.ValuesDelete(webhookHandlerCAPath)
				f.KubeStateSet(createdSecret)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("should restore certificates from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(webhookHandlerCertPath).String()).ToNot(BeEmpty())
				Expect(f.ValuesGet(webhookHandlerKeyPath).String()).ToNot(BeEmpty())
				Expect(f.ValuesGet(webhookHandlerCAPath).String()).ToNot(BeEmpty())
			})
		})
	})
})

func createWebhookSecret(ca, crt, key string) string {
	return fmt.Sprintf(
		`
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-config-webhook-tls
  namespace: %s
data:
  tls.crt: %s
  tls.key: %s
  ca.crt:  %s
`, webhookServiceNamespace, base64.StdEncoding.EncodeToString([]byte(crt)), base64.StdEncoding.EncodeToString([]byte(key)), base64.StdEncoding.EncodeToString([]byte(ca)))
}
