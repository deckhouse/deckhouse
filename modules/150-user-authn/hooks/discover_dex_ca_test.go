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

var _ = Describe("User Authn hooks :: discover dex ca ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{"controlPlaneConfigurator":{}}, "controlPlaneConfigurator":{"enabled":true}, "https": {"mode":"CertManager"}}}`, "")

	Context("With FromIngressSecret option and empty cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "FromIngressSecret")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Adding secret", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls
  namespace: d8-user-authn
data:
  tls.crt: dGVzdA==
`, 2))
				f.RunHook()
			})

			It("Should add ca for OIDC provider", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("test"))
			})
		})

		Context("Adding secret with empty ca.crt", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls
  namespace: d8-user-authn
data:
  ca.crt: ""
  tls.crt: dGVzdA==
`, 2))
				f.RunHook()
			})

			It("Should add tls.crt for OIDC provider", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("test"))
			})
		})
	})

	Context("With DoNotNeed option", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "DoNotNeed")
			f.RunHook()
		})
		It("Should add no ca for OIDC provider", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal(""))
		})
	})

	Context("With Custom option and ca in config", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "Custom")
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCustomCA", "testca")
			f.RunHook()
		})
		It("Should add no ca for OIDC provide from config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("testca"))
		})
	})
})
