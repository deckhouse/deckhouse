/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: system-registry :: hooks :: get_pki ::", func() {
	const (
		stateSecret = `
---
apiVersion: v1
data:
  ca.crt: test
  ca.key: test
  front-proxy-ca.crt: test
  front-proxy-ca.key: test
  sa.pub: test
  sa.key: test
  etcd-ca.crt: test
  etcd-ca.key: test
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
`
		stateSecretModified = `
---
apiVersion: v1
data:
  ca.crt: testtest
  ca.key: testtest
  front-proxy-ca.crt: testtest
  front-proxy-ca.key: testtest
  sa.pub: testtest
  sa.key: testtest
  etcd-ca.crt: testtest
  etcd-ca.key: testtest
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
`
	)

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"systemRegistry":{"internal": {"pki": {"data": []}}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).Should(BeEquivalentTo(`there is no Secret named "d8-pki" in NS "kube-system"`))
		})

		Context("Someone added d8-cloud-instance-manager-cloud-provider", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecret))
				f.RunHook()
			})

			It("systemRegistry.internal.pki.data must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("systemRegistry.internal.pki.data").String()).To(MatchJSON(`
[
  {"key":"ca.crt","value":"test"},
  {"key":"ca.key","value":"test"},
  {"key":"etcd-ca.crt","value":"test"},
  {"key":"etcd-ca.key","value":"test"},
  {"key":"front-proxy-ca.crt","value":"test"},
  {"key":"front-proxy-ca.key","value":"test"},
  {"key":"sa.key","value":"test"},
  {"key":"sa.pub","value":"test"}
]
`))
			})
		})
	})

	Context("Secret d8-cloud-instance-manager-cloud-provider is in cluster", func() {
		f := HookExecutionConfigInit(`{"systemRegistry":{"internal": {"pki": {"data": []}}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecret))
			f.RunHook()
		})

		It("systemRegistry.internal.pki.data must be filled with data from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("systemRegistry.internal.pki.data").String()).To(MatchJSON(`
[
  {"key":"ca.crt","value":"test"},
  {"key":"ca.key","value":"test"},
  {"key":"etcd-ca.crt","value":"test"},
  {"key":"etcd-ca.key","value":"test"},
  {"key":"front-proxy-ca.crt","value":"test"},
  {"key":"front-proxy-ca.key","value":"test"},
  {"key":"sa.key","value":"test"},
  {"key":"sa.pub","value":"test"}
]
`))
		})

		Context("Secret d8-pki was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretModified))
				f.RunHook()
			})

			It("systemRegistry.internal.pki.data must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("systemRegistry.internal.pki.data").String()).To(MatchJSON(`
[
  {"key":"ca.crt","value":"testtest"},
  {"key":"ca.key","value":"testtest"},
  {"key":"etcd-ca.crt","value":"testtest"},
  {"key":"etcd-ca.key","value":"testtest"},
  {"key":"front-proxy-ca.crt","value":"testtest"},
  {"key":"front-proxy-ca.key","value":"testtest"},
  {"key":"sa.key","value":"testtest"},
  {"key":"sa.pub","value":"testtest"}
]
`))
			})
		})

		Context("Secret d8-pki was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})

			It("systemRegistry.internal.pki.data must be filled with data from secret", func() {
				Expect(f).ToNot(ExecuteSuccessfully())
				Expect(f.GoHookError.Error()).Should(BeEquivalentTo(`there is no Secret named "d8-pki" in NS "kube-system"`))
			})
		})
	})
})
