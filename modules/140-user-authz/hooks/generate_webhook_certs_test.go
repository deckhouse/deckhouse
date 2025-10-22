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
Webhook mechanism requires a pair of certificates. This hook generates them and stores in cluster as Secret resource.

*/

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"userAuthz":{"enableMultiTenancy": false, "internal":{}}}`
	initConfigValuesString = `{}`
)

const (
	stateSecretCreated = `
apiVersion: v1
kind: Secret
metadata:
  name: user-authz-webhook
  namespace: d8-user-authz
data:
  ca.crt: YQo= # a
  tls.crt: Ygo= # b
  tls.key: Ywo= # c
`

	stateSecretChanged = `
apiVersion: v1
kind: Secret
metadata:
  name: user-authz-webhook
  namespace: d8-user-authz
data:
  ca.crt: eAo= # x
  tls.crt: eQo= # y
  tls.key: ego= # z
`
)

var _ = Describe("User Authz hooks :: gen webhook certs ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Secret Created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
				f.RunHook()
			})

			It("Cert data must be stored in values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.ca").String()).To(Equal("a\n"))
				Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.crt").String()).To(Equal("b\n"))
				Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.key").String()).To(Equal("c\n"))
			})

			Context("Secret Changed", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateSecretChanged))
					f.RunHook()
				})

				It("New cert data must be stored in values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.ca").String()).To(Equal("x\n"))
					Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.crt").String()).To(Equal("y\n"))
					Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.key").String()).To(Equal("z\n"))
				})
			})
		})
	})

	Context("Cluster with secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecretCreated))
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.ca").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.crt").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.key").String()).To(Equal("c\n"))
		})
	})

	Context("Empty cluster with multitenancy, onBeforeHelm", func() {
		BeforeEach(func() {
			// TODO we need to unset cluster state between contexts.
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("userAuthz.internal.webhookCertificate.key").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("userAuthz.internal.webhookCertificate.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("userAuthz.internal.webhookCertificate.crt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "127.0.0.1",
				Roots:   certPool,
			}

			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
