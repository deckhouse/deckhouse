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

var _ = Describe("Istio hooks :: generate_crl ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{"ca":{}}}}`, "")

	Context("No CA material and no settings.crl", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Should leave istio.internal.crl unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.crl").Exists()).To(BeFalse())
		})
	})

	Context("settings.crl.pem is set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.crl", []byte(`
pem: my-operator-crl
`))
			f.ValuesSet("istio.internal.ca.cert", "ignored-for-override")
			f.ValuesSet("istio.internal.ca.key", "ignored-for-override")
			f.RunHook()
		})

		It("Should prefer ModuleConfig settings.crl.pem", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(strings.TrimSpace(f.ValuesGet("istio.internal.crl").String())).To(Equal("my-operator-crl"))
		})
	})

	Context("settings.crl.pem set; federation peer CRL is set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.crl", []byte(`
pem: |
  -----BEGIN X509 CRL-----
  LOCAL
  -----END X509 CRL-----
`))
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
			f.RunHook()
		})

		It("Should append peer CRL after local settings.crl.pem", func() {
			Expect(f).To(ExecuteSuccessfully())
			crlPEM := f.ValuesGet("istio.internal.crl").String()
			Expect(crlPEM).To(ContainSubstring("LOCAL"))
			Expect(crlPEM).To(ContainSubstring("PEERCRL"))
		})
	})

	Context("internal CA present; no settings.crl", func() {
		BeforeEach(func() {
			ca := mustTestCA()
			f.ValuesSet("istio.internal.ca.cert", ca.Cert)
			f.ValuesSet("istio.internal.ca.key", ca.Key)
			f.RunHook()
		})

		It("Should leave istio.internal.crl unset (no auto-dummy)", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.crl").Exists()).To(BeFalse())
		})
	})

	Context("peer CRL present but settings.crl absent", func() {
		BeforeEach(func() {
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
			f.RunHook()
		})

		It("Should not publish peer CRL alone", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.crl").Exists()).To(BeFalse())
		})
	})
})
