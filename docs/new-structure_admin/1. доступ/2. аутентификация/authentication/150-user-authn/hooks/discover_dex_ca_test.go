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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: discover dex ca ::", func() {
	f := HookExecutionConfigInit(`{"userAuthn":{"internal":{},"controlPlaneConfigurator":{"enabled":true}, "https": {"mode":"CertManager"}}}`, "")

	Context("With FromIngressSecret option and empty cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "FromIngressSecret")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Adding ingress-tls secret", func() {
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

		Context("Adding ingress-tls secret with empty ca.crt", func() {
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

		It("Should add no ca for OIDC provider from config", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("testca"))
		})
	})

	Context("Checking for CertManager and CustomcCertificate cases with secrets", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthn.controlPlaneConfigurator.dexCAMode", "FromIngressSecret")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Matching with CertManager and ingress-tls", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthn.https.mode", "CertManager")
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

			It("Should add tls.crt for OIDC provider", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("test"))
			})

			It("Should check non-existing ingress-tls-customcertificate secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls-customcertificate").Exists()).To(BeFalse())
			})
		})

		Context("Matching with CustomCertificate and ingress-tls-customcertificate", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthn.https.mode", "CustomCertificate")
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls-customcertificate
  namespace: d8-user-authn
data:
  tls.crt: dGVzdGNh
`, 2))
				f.RunHook()
			})

			It("Should add tls.crt for OIDC provider", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("testca"))
			})

			It("Should check non-existing ingress-tls secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls").Exists()).To(BeFalse())
			})
		})

		Context("Incorrect matching between mode and secret", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthn.https.mode", "CustomCertificate")
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

			It("Should generate an error", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(fmt.Errorf("cannot convert dex ca certificate from snaphots"))
			})
		})

		Context("Proper matching between mode and one of existing secrets", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthn.https.mode", "CustomCertificate")
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
kind: Secret
metadata:
  name: ingress-tls-customcertificate
  namespace: d8-user-authn
data:
  tls.crt: dGVzdGNh
---
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

			It("Should use only ingress-tls-customcertificate secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.https.mode").String()).Should(BeEquivalentTo("CustomCertificate"))
				Expect(f.KubernetesResource("Secret", "d8-user-authn", "ingress-tls-customcertificate").Exists()).To(BeTrue())
				Expect(f.ValuesGet("userAuthn.internal.discoveredDexCA").String()).To(Equal("testca"))
			})
		})
	})
})
