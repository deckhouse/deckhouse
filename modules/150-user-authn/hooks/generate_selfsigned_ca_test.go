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
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: generate selfsigned ca ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{"selfSignedCA":{}}}}`, "")

	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("userAuthn.publishAPI.enabled", true)
			f.ValuesSet("userAuthn.publishAPI.https.mode", "SelfSigned")
			f.RunHook()
		})

		It("Should add ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.selfSignedCA.key").Exists()).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("userAuthn.internal.selfSignedCA.cert").String()))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.CommonName).To(Equal("kubernetes-api-selfsigned-ca"))
		})
	})
	Context("Without secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-api-ca-key-pair
  namespace: d8-user-authn
data:
  tls.crt: dGVzdA==
  tls.key: dGVzdA==
`))
			f.ValuesSet("userAuthn.publishAPI.enabled", true)
			f.ValuesSet("userAuthn.publishAPI.https.mode", "SelfSigned")
			f.RunHook()
		})
		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("userAuthn.internal.selfSignedCA.cert").String()).To(Equal("test"))
			Expect(f.ValuesGet("userAuthn.internal.selfSignedCA.key").String()).To(Equal("test"))
		})

	})
})
