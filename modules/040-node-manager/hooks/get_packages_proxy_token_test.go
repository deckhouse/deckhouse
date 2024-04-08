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

const tokenSecret = `
apiVersion: v1
data:
  token: QUFBQUFBQUFBQUE= # AAAAAAAAAAA
kind: Secret
metadata:
  annotations:
    kubernetes.io/service-account.name: registry-packages-proxy-reader
  name: registry-packages-proxy-reader-token
  namespace: d8-cloud-instance-manager
type: kubernetes.io/service-account-token
`

var _ = FDescribe("Modules :: node-group :: hooks :: get_packages_proxy_token ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)

	Context("Cluster is empty", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must not fail, token should be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxyToken").String()).To(Equal(""))
		})
	})

	Context("Cluster has token", func() {
		BeforeEach(func() {
			f.KubeStateSet(tokenSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must not fail, token should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.packagesProxyToken").String()).To(Equal("AAAAAAAAAAA"))
		})
	})

})
