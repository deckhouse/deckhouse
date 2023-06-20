/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: alliance_generate_authn_keypair ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{"remoteAuthnKeypair":{}}}}`, "")

	Context("Empty cluster; empty values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate keypair", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.remoteAuthnKeypair.priv").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.remoteAuthnKeypair.pub").Exists()).To(BeTrue())

			privString := f.ValuesGet("istio.internal.remoteAuthnKeypair.priv").String()
			pubString := f.ValuesGet("istio.internal.remoteAuthnKeypair.pub").String()

			privBlock, _ := pem.Decode([]byte(privString))
			_, err0 := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
			Expect(err0).To(BeNil())

			pubBlock, _ := pem.Decode([]byte(pubString))
			_, err1 := x509.ParsePKIXPublicKey(pubBlock.Bytes)
			Expect(err1).To(BeNil())
		})
	})

	Context("Secret d8-remote-authn-keypair is in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-remote-authn-keypair
  namespace: d8-istio
data:
  pub.pem: YWFh # aaa
  priv.pem: YmJi # bbb
`))
			f.RunHook()
		})
		It("Should add existing key to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.remoteAuthnKeypair.pub").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.remoteAuthnKeypair.priv").String()).To(Equal("bbb"))
		})
	})
})
