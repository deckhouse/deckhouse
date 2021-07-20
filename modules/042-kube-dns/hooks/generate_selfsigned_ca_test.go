/*
Copyright 2021 Flant CJSC

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

const (
	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-kube-dns-sts-pods-hosts-appender-webhook
  namespace: kube-system
data:
  tls.crt: YQ== # a
  tls.key: Yg== # b
  ca.crt:  Yw== # c
`
)

var _ = Describe("KubeDns hooks :: generate_selfsigned_ca ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.ca").Exists()).To(BeTrue())

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
		})

		Context("Secret Created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
				f.RunHook()
			})

			It("Cert data must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.crt").String()).To(Equal("a"))
				Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.key").String()).To(Equal("b"))
				Expect(f.ValuesGet("kubeDns.internal.stsPodsHostsAppenderWebhook.ca").String()).To(Equal("c"))
			})
		})
	})
})
