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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: credentials ::", func() {
	const initValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal: {}
`

	credSecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-yandex
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: %s
  secret: %s
`, base64.StdEncoding.EncodeToString([]byte("serviceAccount")), base64.StdEncoding.EncodeToString([]byte("sa-secret-data")))

	credSecretExporter := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials-exporter
  namespace: d8-cloud-provider-yandex
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: %s
  secret: %s
`, base64.StdEncoding.EncodeToString([]byte("apiToken")), base64.StdEncoding.EncodeToString([]byte("exporter-token")))

	// Secret with wrong type — should be ignored.
	wrongTypeSecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: some-other-secret
  namespace: d8-cloud-provider-yandex
type: Opaque
data:
  key: %s
`, base64.StdEncoding.EncodeToString([]byte("value")))

	Context("No credential secrets", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("should result in empty credentialSecrets", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(0))
		})
	})

	Context("One valid credential secret (d8-credentials)", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(credSecret))
			f.RunHook()
		})

		It("should populate credentialSecrets with one entry", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(1))

			entry := secrets.Get("d8-credentials")
			Expect(entry.Get("authScheme").String()).To(Equal("serviceAccount"))
			Expect(entry.Get("secret").String()).To(Equal("sa-secret-data"))
		})
	})

	Context("Two credential secrets (d8-credentials + d8-credentials-exporter)", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(credSecret + "\n---\n" + credSecretExporter))
			f.RunHook()
		})

		It("should populate credentialSecrets with both entries", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(2))

			entry1 := secrets.Get("d8-credentials")
			Expect(entry1.Get("authScheme").String()).To(Equal("serviceAccount"))
			Expect(entry1.Get("secret").String()).To(Equal("sa-secret-data"))

			entry2 := secrets.Get("d8-credentials-exporter")
			Expect(entry2.Get("authScheme").String()).To(Equal("apiToken"))
			Expect(entry2.Get("secret").String()).To(Equal("exporter-token"))
		})
	})

	Context("Secret with wrong type", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(wrongTypeSecret))
			f.RunHook()
		})

		It("should be ignored and result in empty credentialSecrets", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(0))
		})
	})
})
