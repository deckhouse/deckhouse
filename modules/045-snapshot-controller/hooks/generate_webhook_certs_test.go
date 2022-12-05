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
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"snapshotController":{"internal":{"webhookCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
	initConfigValuesString = `{}`
)

var _ = Describe("Modules :: snapshot-controller :: hooks :: generate_certs ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(``),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("snapshotController.internal.webhookCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("snapshotController.internal.webhookCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("snapshotController.internal.webhookCert.ca").Exists()).To(BeTrue())

			// controller certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("snapshotController.internal.webhookCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("snapshotController.internal.webhookCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// Additional checks for controller certificate
			opts := x509.VerifyOptions{
				DNSName: "snapshot-validation-webhook.d8-snapshot-controller.svc",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

			opts = x509.VerifyOptions{
				DNSName: "127.0.0.1",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

		})
	})

	Context("Secret Created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: snapshot-validation-webhook-certs
  namespace: d8-snapshot-controller
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("snapshotController.internal.webhookCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("snapshotController.internal.webhookCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("snapshotController.internal.webhookCert.ca").String()).To(Equal("c\n"))
		})
	})
})
