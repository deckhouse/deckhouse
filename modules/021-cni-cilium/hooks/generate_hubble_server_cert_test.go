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
	"github.com/cloudflare/cfssl/csr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: generate_ca", func() {
	f := HookExecutionConfigInit(
		`{"cniCilium": {"internal": {"hubble": {"certs": {"ca":{}, "server": {}}}}} }`,
		`{"cniCilium":{}}`,
	)
	const cn = "d8.hubble-ca.cilium.io"
	ca, _ := certificate.GenerateCA(&logrus.Entry{}, cn,
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithGroups("d8-cni-cilium"),
	)

	Context("fresh cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("cniCilium.internal.hubble.certs.ca.cert", ca.Cert)
			f.ValuesSet("cniCilium.internal.hubble.certs.ca.key", ca.Key)
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should generate new server certs", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cniCilium.internal.hubble.certs.server.key").String()).ToNot(BeEmpty())
			Expect(f.ValuesGet("cniCilium.internal.hubble.certs.server.ca").String()).ToNot(BeEmpty())
			Expect(f.ValuesGet("cniCilium.internal.hubble.certs.server.cert").String()).ToNot(BeEmpty())
		})
	})
})
