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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString = `{
  "kubeDns": {
    "clusterDomainAliases": ["abc"],
    "internal": {
      "stsPodsHostsAppenderWebhook":{}
    }
  }
}`
	initConfigValuesString = `{}`
)

var _ = Describe("KubeDns hooks :: generate_selfsigned_ca ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		var createdSecret string
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			caValue := f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.ca")
			keyValue := f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.key")
			certValue := f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.crt")
			Expect(certValue.Exists()).To(BeTrue())
			Expect(keyValue.Exists()).To(BeTrue())
			Expect(caValue.Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.crt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "d8-kube-dns-sts-pods-hosts-appender-webhook.kube-system.svc",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
			createdSecret = testGenerateSecret(caValue.String(), certValue.String(), keyValue.String())
		})

		Context("Secret Created", func() {
			BeforeEach(func() {
				f.ValuesDelete("kubeDns.internal.stsPodsHostsAppenderWebhook.ca")
				f.ValuesDelete("kubeDns.internal.stsPodsHostsAppenderWebhook.crt")
				f.ValuesDelete("kubeDns.internal.stsPodsHostsAppenderWebhook.key")
				f.BindingContexts.Set(f.KubeStateSet(createdSecret))
				f.RunHook()
			})

			It("Cert data must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.crt").String()).ToNot(BeEmpty())
				Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.key").String()).ToNot(BeEmpty())
				Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.ca").String()).ToNot(BeEmpty())
			})
		})
	})

	Context("clusterDomainAliases not set", func() {
		BeforeEach(func() {
			f.ValuesDelete("kubeDns.clusterDomainAliases")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Certificate should not be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.crt").String()).To(BeEmpty())
			Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.key").String()).To(BeEmpty())
			Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.ca").String()).To(BeEmpty())
		})
	})
})

func testGenerateSecret(ca, crt, key string) string {
	return fmt.Sprintf(
		`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-kube-dns-sts-pods-hosts-appender-webhook
  namespace: kube-system
data:
  tls.crt: %s
  tls.key: %s
  ca.crt:  %s
`, base64.StdEncoding.EncodeToString([]byte(crt)), base64.StdEncoding.EncodeToString([]byte(key)), base64.StdEncoding.EncodeToString([]byte(ca)))
}
