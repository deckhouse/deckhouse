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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type inputPublishAPICACert struct {
	manifests          string
	httpsMode          string
	publishAPIMode     string
	kubeconfigMasterCA *string
}

var _ = Describe("Control plane manager hooks :: discover publish api cert ::", func() {
	f := HookExecutionConfigInit(
		`
global:
  modules:
    https:
      mode: CertManager
  discovery:
    kubernetesCA: "discoveredKubernetesCA"
controlPlaneManager:
  apiserver:
    publishAPI:
      ingress:
        enabled: true
        https:
          mode: SelfSigned
  internal:
    authn: {}
`,
		"",
	)
	selfSignedCertSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls-selfsigned
  namespace: kube-system
data:
  ca.crt: a3ViZXJuZXRlcy10bHMtc2VsZnNpZ25lZA==
`
	certManagerCertSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: kube-system
data:
  ca.crt: a3ViZXJuZXRlcy10bHM=
`
	certManagerCertSecretRotated := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: kube-system
data:
  ca.crt: a3ViZXJuZXRlcy10bHMtcm90YXRlZA==
`
	customCertSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls-customcertificate
  namespace: kube-system
data:
  ca.crt: a3ViZXJuZXRlcy10bHMtY3VzdG9tY2VydGlmaWNhdGU=
`

	DescribeTable("publishAPI discovery cert",
		func(in inputPublishAPICACert, out string) {
			f.KubeStateSet(in.manifests)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", in.publishAPIMode)
			f.ValuesSet("global.modules.https.mode", in.httpsMode)

			if in.kubeconfigMasterCA != nil {
				f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.global.kubeconfigGeneratorMasterCA", *in.kubeconfigMasterCA)
			}

			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal(out))
		},
		Entry("On first start: SelfSigned",
			inputPublishAPICACert{
				manifests:      "",
				publishAPIMode: "SelfSigned",
				httpsMode:      "CertManager",
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: SelfSigned",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "SelfSigned",
				httpsMode:      "CertManager",
			},
			"kubernetes-tls-selfsigned",
		),
		Entry("Without secret for self signed: SelfSigned",
			inputPublishAPICACert{
				manifests:      certManagerCertSecret + customCertSecret,
				publishAPIMode: "SelfSigned",
				httpsMode:      "CertManager",
			},
			"discoveredKubernetesCA",
		),
		Entry("On first start: SelfSigned",
			inputPublishAPICACert{
				manifests:          "",
				publishAPIMode:     "SelfSigned",
				httpsMode:          "CertManager",
				kubeconfigMasterCA: ptr.To("test"),
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: Global: CertManager",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpsMode:      "CertManager",
			},
			"kubernetes-tls",
		),
		Entry("Without secret for cert manager: Global: CertManager",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpsMode:      "CertManager",
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: Global: CustomCertificate",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpsMode:      "CustomCertificate",
			},
			"kubernetes-tls-customcertificate",
		),
		Entry("Without secret for custom certificate: Global: CustomCertificate",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret,
				publishAPIMode: "Global",
				httpsMode:      "CustomCertificate",
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: Global: OnlyInURI",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpsMode:      "OnlyInURI",
			},
			"discoveredKubernetesCA",
		),
		Entry("Without secret: Global: OnlyInURI",
			inputPublishAPICACert{
				manifests:      "",
				publishAPIMode: "Global",
				httpsMode:      "OnlyInURI",
			},
			"discoveredKubernetesCA",
		),
		Entry("Without secret: Global: OnlyInURI with custom CA",
			inputPublishAPICACert{
				manifests:          "",
				publishAPIMode:     "Global",
				httpsMode:          "OnlyInURI",
				kubeconfigMasterCA: ptr.To("testMasterCA"),
			},
			"testMasterCA",
		),
		Entry("Without secret: Global: OnlyInURI with custom CA empty",
			inputPublishAPICACert{
				manifests:          "",
				publishAPIMode:     "Global",
				httpsMode:          "OnlyInURI",
				kubeconfigMasterCA: ptr.To(""),
			},
			"",
		),
	)

	Context("With all secrets in cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "SelfSigned")
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.KubeStateSet(selfSignedCertSecret + certManagerCertSecret + customCertSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should delete not matching secrets", func() {
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls-selfsigned").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("Global mode with CertManager and all secrets in cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "Global")
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.KubeStateSet(selfSignedCertSecret + certManagerCertSecret + customCertSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should keep kubernetes-tls and delete not matching secrets", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal("kubernetes-tls"))
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls-selfsigned").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("Global mode with CertManager, kubernetes-tls changes after BeforeHelm", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "Global")
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.KubeStateSet(certManagerCertSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
			f.BindingContexts.Set(f.KubeStateSet(certManagerCertSecretRotated))
			f.RunHook()
		})

		It("Should publish the new CA on the secret event", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.authn.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal("kubernetes-tls-rotated"))
		})
	})

	Context("Global mode with kubeconfigGeneratorMasterCA set and all secrets in cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "Global")
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.global.kubeconfigGeneratorMasterCA", "")
			f.KubeStateSet(selfSignedCertSecret + certManagerCertSecret + customCertSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should keep every secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls-selfsigned").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeTrue())
		})
	})
})
