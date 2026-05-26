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

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: credentials ::", func() {
	const (
		initValues = `
global:
  discovery: {}
cloudProviderDvp:
  internal:
    credentialSecrets: {}
`
	)

	// S3ViZWNvbmZpZw== = base64("kubeconfig"), YXBpVmV= = base64("apiVe")
	credSecret1 := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: S3ViZWNvbmZpZw==
  secret: YXBpVmV=
`

	// dXNlcnBhc3M= = base64("userpass"), dXNlcjE= = base64("user1"), cGFzczE= = base64("pass1")
	credSecret2 := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials-extra
  namespace: d8-cloud-provider-dvp
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
data:
  authScheme: dXNlcnBhc3M=
  identity: dXNlcjE=
  secret: cGFzczE=
`

	nonCredSecret := `
apiVersion: v1
kind: Secret
metadata:
  name: some-other-secret
  namespace: d8-cloud-provider-dvp
type: Opaque
data:
  key: dmFsdWU=
`

	Context("Single credential secret", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(credSecret1))
			f.RunHook()
		})

		It("should populate credentialSecrets with one entry", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(1))

			entry := secrets.Get("d8-credentials")
			// S3ViZWNvbmZpZw== decodes to "Kubeconfig"
			Expect(entry.Get("authScheme").String()).To(Equal("Kubeconfig"))
			// YXBpVmV= decodes to "apiVe"
			Expect(entry.Get("secret").String()).To(Equal("apiVe"))
			Expect(entry.Get("identity").Exists()).To(BeFalse())
		})
	})

	Context("Two credential secrets", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(credSecret1 + "\n---\n" + credSecret2))
			f.RunHook()
		})

		It("should populate credentialSecrets with two entries", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(2))

			entry1 := secrets.Get("d8-credentials")
			// S3ViZWNvbmZpZw== decodes to "Kubeconfig"
			Expect(entry1.Get("authScheme").String()).To(Equal("Kubeconfig"))

			entry2 := secrets.Get("d8-credentials-extra")
			Expect(entry2.Get("authScheme").String()).To(Equal("userpass"))
			Expect(entry2.Get("identity").String()).To(Equal("user1"))
			Expect(entry2.Get("secret").String()).To(Equal("pass1"))
		})
	})

	Context("Non-credential secret is ignored", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nonCredSecret))
			f.RunHook()
		})

		It("should result in empty credentialSecrets", func() {
			Expect(f).To(ExecuteSuccessfully())

			secrets := f.ValuesGet("cloudProviderDvp.internal.credentialSecrets")
			Expect(secrets.Map()).To(HaveLen(0))
		})
	})

	Context("No secrets", func() {
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
})
