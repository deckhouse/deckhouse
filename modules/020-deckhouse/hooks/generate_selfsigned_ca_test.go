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

/*

User-stories:
1. Webhook mechanism requires a pair of certificates. This hook generates them and stores in values.

*/

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"deckhouse":{"internal":{"webhookHandlerCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
	initConfigValuesString = `{}`
)

const (
	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: webhook-handler-certs
  namespace: d8-system
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
`
)

var _ = Describe("Deckhouse hooks :: generate_selfsigned_ca ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "webhook-handler.d8-system.svc.mycluster.local",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("Secret Created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()).To(Equal("c\n"))
		})
	})

	Context("Before Helm", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateSecretCreated)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()).To(Equal("c\n"))
		})
	})
})
