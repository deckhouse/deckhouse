// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. There is CM kube-system/extension-apiserver-authentication with CA for verification requests to our custom modules from clients inside cluster, hook must store it to `global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA`.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: extension_api_server_authentication_test ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateA = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-apiserver-authentication
  namespace: kube-system
data:
  requestheader-client-ca-file: |
    qraga
`
		stateB = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: extension-apiserver-authentication
  namespace: kube-system
data:
  requestheader-client-ca-file: |
    pickle
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Cluster started with some extension-apiserver-authentication", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateA))
			f.RunHook()
		})

		It("clientCA must be 'qraga'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA").String()).To(Equal("qraga"))
		})

		Context("extension-apiserver-authentication changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateB))
				f.RunHook()
			})

			It("clientCA must be 'pickle'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA").String()).To(Equal("pickle"))
			})

		})

	})
})
