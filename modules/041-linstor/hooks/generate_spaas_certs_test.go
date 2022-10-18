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
	initSpaasValuesString = `{"linstor":{"internal":{"spaasCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
)

var _ = Describe("Modules :: linstor :: hooks :: generate_spaas_certs ::", func() {
	f := HookExecutionConfigInit(initSpaasValuesString, initConfigValuesString)

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
			Expect(f.ValuesGet("linstor.internal.spaasCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.spaasCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.spaasCert.ca").Exists()).To(BeTrue())

			// controller certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.spaasCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.spaasCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// Additional checks for controller certificate
			opts := x509.VerifyOptions{
				DNSName: "spaas.d8-linstor.svc",
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
  name: spaas-certs
  namespace: d8-linstor
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
			Expect(f.ValuesGet("linstor.internal.spaasCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.spaasCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.spaasCert.ca").String()).To(Equal("c\n"))
		})
	})
})
