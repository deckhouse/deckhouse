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

var _ = Describe("Modules :: control-plane-manager :: hooks :: get_pki_checksum ::", func() {
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
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

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

			It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.pkiChecksum").String()).To(Equal("825ed27a80e5d85b15b0a7a00d83e9635da552634a5af66b17d82ba4d1e547ef"))
			})
		})
	})

	Context("Secret d8-cloud-instance-manager-cloud-provider is in cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecret))
			f.RunHook()
		})

		It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.pkiChecksum").String()).To(Equal("825ed27a80e5d85b15b0a7a00d83e9635da552634a5af66b17d82ba4d1e547ef"))
		})

		Context("Secret d8-pki was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretModified))
				f.RunHook()
			})

			It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.pkiChecksum").String()).To(Equal("27de60471f8132fbab61bc9020ccdeb9dc88b58b482b4678b208899e22193e2c"))
			})
		})

		Context("Secret d8-pki was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})

			It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
				Expect(f).ToNot(ExecuteSuccessfully())
				Expect(f.GoHookError.Error()).Should(BeEquivalentTo(`there is no Secret named "d8-pki" in NS "kube-system"`))
			})
		})
	})
})
