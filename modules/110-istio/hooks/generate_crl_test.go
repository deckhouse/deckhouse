/*
Copyright 2026 Flant JSC

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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func mustTestCA() certificate.Authority {
	ca, err := certificate.GenerateCA(log.NewNop(), "d8-istio-crl-test")
	Expect(err).NotTo(HaveOccurred())
	return ca
}

func mustParseCRL(pemStr string) *x509.RevocationList {
	block, _ := pem.Decode([]byte(pemStr))
	Expect(block).NotTo(BeNil())
	Expect(block.Type).To(Equal("X509 CRL"))
	crl, err := x509.ParseRevocationList(block.Bytes)
	Expect(err).NotTo(HaveOccurred())
	return crl
}

var _ = Describe("Istio hooks :: generate_crl ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{"ca":{}}}}`, "")

	Context("No CA material and no settings.crl", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should leave istio.internal.crl unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.crl").Exists()).To(BeFalse())
		})
	})

	Context("settings.crl is set", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.crl", "my-operator-crl")
			f.ValuesSet("istio.internal.ca.cert", "ignored-for-override")
			f.ValuesSet("istio.internal.ca.key", "ignored-for-override")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should prefer ModuleConfig settings.crl", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(strings.TrimSpace(f.ValuesGet("istio.internal.crl").String())).To(Equal("my-operator-crl"))
		})
	})

	Context("internal CA present; federation peer CRL is set", func() {
		var ca certificate.Authority

		BeforeEach(func() {
			ca = mustTestCA()
			f.ValuesSet("istio.internal.ca.cert", ca.Cert)
			f.ValuesSet("istio.internal.ca.key", ca.Key)
			f.ValuesSetFromYaml("istio.internal.federations", []byte(`
- name: peer
  clusterUUID: uuid-peer
  trustDomain: peer.local
  rootCA: ---PEER-ROOT---
  crl: |
    -----BEGIN X509 CRL-----
    PEERCRL
    -----END X509 CRL-----
`))
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should append peer CRL after local dummy", func() {
			Expect(f).To(ExecuteSuccessfully())
			crlPEM := f.ValuesGet("istio.internal.crl").String()
			Expect(crlPEM).To(ContainSubstring("BEGIN X509 CRL"))
			Expect(crlPEM).To(ContainSubstring("PEERCRL"))
			// First block is local dummy signed by our CA.
			block, rest := pem.Decode([]byte(crlPEM))
			Expect(block).NotTo(BeNil())
			localCRL, err := x509.ParseRevocationList(block.Bytes)
			Expect(err).NotTo(HaveOccurred())
			certBlock, _ := pem.Decode([]byte(ca.Cert))
			cert, err := x509.ParseCertificate(certBlock.Bytes)
			Expect(err).NotTo(HaveOccurred())
			Expect(localCRL.CheckSignatureFrom(cert)).To(Succeed())
			Expect(string(rest)).To(ContainSubstring("PEERCRL"))
		})
	})

	Context("internal CA present; no settings.crl; no secret CRL", func() {
		var ca certificate.Authority

		BeforeEach(func() {
			ca = mustTestCA()
			f.ValuesSet("istio.internal.ca.cert", ca.Cert)
			f.ValuesSet("istio.internal.ca.key", ca.Key)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should mint an empty dummy CRL signed by internal CA", func() {
			Expect(f).To(ExecuteSuccessfully())
			crlPEM := f.ValuesGet("istio.internal.crl").String()
			Expect(crlPEM).NotTo(BeEmpty())

			crl := mustParseCRL(crlPEM)
			Expect(crl.RevokedCertificateEntries).To(BeEmpty())

			block, _ := pem.Decode([]byte(ca.Cert))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred())
			Expect(crl.CheckSignatureFrom(cert)).To(Succeed())
		})
	})

	Context("internal CA present; secret has CRL signed by that CA", func() {
		var ca certificate.Authority
		var existingCRL string

		BeforeEach(func() {
			ca = mustTestCA()
			cert, signer, err := parseCACertAndKey(ca.Cert, ca.Key)
			Expect(err).NotTo(HaveOccurred())
			existingCRL, err = createEmptyCRL(cert, signer)
			Expect(err).NotTo(HaveOccurred())

			f.ValuesSet("istio.internal.ca.cert", ca.Cert)
			f.ValuesSet("istio.internal.ca.key", ca.Key)
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-crl.pem: ` + base64.StdEncoding.EncodeToString([]byte(existingCRL)) + `
`))
			f.RunHook()
		})

		It("Should keep the existing cacerts CRL to avoid churn", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(strings.TrimSpace(f.ValuesGet("istio.internal.crl").String())).To(Equal(strings.TrimSpace(existingCRL)))
		})
	})

	Context("internal CA present; secret has local+peer CRL mix", func() {
		var ca, peerCA certificate.Authority
		var localCRL, peerCRL string

		BeforeEach(func() {
			ca = mustTestCA()
			peerCA = mustTestCA()
			cert, signer, err := parseCACertAndKey(ca.Cert, ca.Key)
			Expect(err).NotTo(HaveOccurred())
			localCRL, err = createEmptyCRL(cert, signer)
			Expect(err).NotTo(HaveOccurred())
			peerCert, peerSigner, err := parseCACertAndKey(peerCA.Cert, peerCA.Key)
			Expect(err).NotTo(HaveOccurred())
			peerCRL, err = createEmptyCRL(peerCert, peerSigner)
			Expect(err).NotTo(HaveOccurred())

			f.ValuesSet("istio.internal.ca.cert", ca.Cert)
			f.ValuesSet("istio.internal.ca.key", ca.Key)
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-crl.pem: ` + base64.StdEncoding.EncodeToString([]byte(localCRL+peerCRL)) + `
`))
			f.RunHook()
		})

		It("Should keep only locally signed blocks from secret when no peers in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(strings.TrimSpace(f.ValuesGet("istio.internal.crl").String())).To(Equal(strings.TrimSpace(localCRL)))
		})
	})

	Context("internal CA present; secret CRL is garbage", func() {
		var ca certificate.Authority

		BeforeEach(func() {
			ca = mustTestCA()
			f.ValuesSet("istio.internal.ca.cert", ca.Cert)
			f.ValuesSet("istio.internal.ca.key", ca.Key)
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-crl.pem: ` + base64.StdEncoding.EncodeToString([]byte("not-a-crl")) + `
`))
			f.RunHook()
		})

		It("Should mint a fresh dummy CRL", func() {
			Expect(f).To(ExecuteSuccessfully())
			crlPEM := f.ValuesGet("istio.internal.crl").String()
			Expect(crlPEM).NotTo(BeEmpty())
			Expect(crlPEM).NotTo(Equal("not-a-crl"))
			_ = mustParseCRL(crlPEM)
		})
	})
})
