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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controlPlaneManager :: hooks :: sync-etcd-certs ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal":{"etcdCerts": {}}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("secret exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pkiState))
			f.RunHook()
		})

		It("Must fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.etcdCerts.ca").String()).To(BeEquivalentTo("YWFh"))
			Expect(f.ValuesGet("controlPlaneManager.internal.etcdCerts.crt").String()).To(BeEquivalentTo("YWFh"))
			Expect(f.ValuesGet("controlPlaneManager.internal.etcdCerts.key").String()).To(BeEquivalentTo("YmJi"))
		})
	})
})

var pkiState = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
data:
  etcd-ca.crt: YWFh # aaa
  etcd-ca.key: YmJi # bbb
`
