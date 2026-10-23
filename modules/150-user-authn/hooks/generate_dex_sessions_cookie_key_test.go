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
	"encoding/base64"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// storedCookieKey is the key of the Secret in the tests: a fixed value of the right length.
var storedCookieKey = strings.Repeat("k", 32)

var _ = Describe("User Authn hooks :: generate dex sessions cookie key ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal": {}}}`, "")

	Context("With the Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-sessions-cookie-key
  namespace: d8-user-authn
data:
  secret: ` + base64.StdEncoding.EncodeToString([]byte(storedCookieKey)) + `
`))
			f.RunHook()
		})

		It("Should keep the stored key", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.dexSessionsCookieKey").String()).To(Equal(storedCookieKey))
		})
	})

	Context("Without the Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Should generate a key of 32 bytes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.dexSessionsCookieKey").String()).To(HaveLen(32))
		})
	})

	Context("With a key Dex would refuse", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: dex-sessions-cookie-key
  namespace: d8-user-authn
data:
  secret: QUJD # ABC
`))
			f.RunHook()
		})

		It("Should generate a key of 32 bytes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.dexSessionsCookieKey").String()).To(HaveLen(32))
		})
	})
})
