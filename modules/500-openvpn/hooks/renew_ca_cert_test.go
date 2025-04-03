/*
Copyright 2025 Flant JSC

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

var _ = Describe("check_server_cert_expiry hook", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("When secret is missing", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.RunHook()
		})

		It("Should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("When tls.crt is missing in the secret", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data: {}
`)
			f.RunHook()
		})

		It("Should not panic and skip", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("When tls.crt contains invalid data", func() {
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: aW52YWxpZCBkYXRhCg==  # "invalid data\n"
`)
			f.RunHook()
		})

		It("Should not panic on invalid certificate", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})
