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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: common :: hooks :: copy_custom_certificate ::", func() {
	const (
		stateNamespaces = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
`
		stateSecrets = `
---
apiVersion: v1
data:
  tls.crt: Q1JUQ1JUQ1JUCg== # CRTCRTCRT
  tls.key: S0VZS0VZS0VZCg== # KEYKEYKEY
kind: Secret
metadata:
  name: d8-tls-cert
  namespace: d8-system
type: kubernetes.io/tls
`
	)

	f := HookExecutionConfigInit(`{"common":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Namespace and secret are in cluster, https mode set to Disabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("common.https.mode", "Disabled")
			f.RunHook()
		})

		It("Module value internal.customCertificateData must be unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.customCertificateData").Exists()).To(BeFalse())
		})

	})

	Context("Namespace and secret are in cluster, https mode set to customCertificate, but certificate name is wrong", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("common.https.mode", "CustomCertificate")
			f.ValuesSet("common.https.customCertificate.secretName", "blablabla")
			f.RunHook()
		})

		It("Hook must generate none certificate data", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.customCertificateData").String()).To(MatchYAML(`
ca.crt: <none>
tls.crt: <none>
tls.key: <none>
`))
			// gbytes.Say panics with Go hooks
			// Expect(f.Session.Err).Should(gbytes.Say(`ERROR: custom certificate secret name is configured, but secret with this name doesn't exist.`))
		})

	})

	Context("Namespace and secret are in cluster, https mode set to customCertificate, certificate name is set correctly", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("common.https.mode", "CustomCertificate")
			f.ValuesSet("common.https.customCertificate.secretName", "d8-tls-cert")
			f.RunHook()
		})

		It("Hook must successfully save certificate data", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.customCertificateData").Exists()).To(BeTrue())
			Expect(f.ValuesGet("common.internal.customCertificateData").String()).To(MatchYAML(`
tls.crt: Q1JUQ1JUQ1JUCg==
tls.key: S0VZS0VZS0VZCg==
`))

		})

	})

})
