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
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type inputPublishAPICACert struct {
	manifests          string
	httpMode           string
	publishAPIMode     string
	kubeconfigMasterCA *string
}

var _ = Describe("User Authn hooks :: discover publish api cert ::", func() {
	f := HookExecutionConfigInit(
		`
global:
  discovery:
    kubernetesCA: "discoveredKubernetesCA"
userAuthn:
  publishAPI:
    enabled: true
    https:
      mode: SelfSigned
  internal: {}
  https:
    mode: CertManager`,
		"",
	)
	selfSignedCertSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls-selfsigned
  namespace: d8-user-authn
data:
  ca.crt: a3ViZXJuZXRlcy10bHMtc2VsZnNpZ25lZA==
`
	certManagerCertSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls
  namespace: d8-user-authn
data:
  ca.crt: a3ViZXJuZXRlcy10bHM=
`
	customCertSecret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: kubernetes-tls-customcertificate
  namespace: d8-user-authn
data:
  ca.crt: a3ViZXJuZXRlcy10bHMtY3VzdG9tY2VydGlmaWNhdGU=
`

	DescribeTable("publishAPI discovery cert",
		func(in inputPublishAPICACert, out string) {
			f.BindingContexts.Set(f.KubeStateSet(in.manifests))
			f.ValuesSet("userAuthn.publishAPI.https.mode", in.publishAPIMode)
			f.ValuesSet("userAuthn.https.mode", in.httpMode)

			if in.kubeconfigMasterCA != nil {
				f.ValuesSet("userAuthn.publishAPI.https.global.kubeconfigGeneratorMasterCA", *in.kubeconfigMasterCA)
			}

			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA").String()).To(Equal(out))
		},
		Entry("On first start: SelfSigned",
			inputPublishAPICACert{
				manifests:      "",
				publishAPIMode: "SelfSigned",
				httpMode:       "CertManager",
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: SelfSigned",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "SelfSigned",
				httpMode:       "CertManager",
			},
			"kubernetes-tls-selfsigned",
		),
		Entry("Without secret for self signed: SelfSigned",
			inputPublishAPICACert{
				manifests:      certManagerCertSecret + customCertSecret,
				publishAPIMode: "SelfSigned",
				httpMode:       "CertManager",
			},
			"discoveredKubernetesCA",
		),
		Entry("On first start: SelfSigned",
			inputPublishAPICACert{
				manifests:          "",
				publishAPIMode:     "SelfSigned",
				httpMode:           "CertManager",
				kubeconfigMasterCA: pointer.String("test"),
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: Global: CertManager",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpMode:       "CertManager",
			},
			"kubernetes-tls",
		),
		Entry("Without secret for cert manager: Global: CertManager",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpMode:       "CertManager",
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: Global: CustomCertificate",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpMode:       "CustomCertificate",
			},
			"kubernetes-tls-customcertificate",
		),
		Entry("Without secret for custom certificate: Global: CustomCertificate",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret,
				publishAPIMode: "Global",
				httpMode:       "CustomCertificate",
			},
			"discoveredKubernetesCA",
		),
		Entry("With every secret: Global: OnlyInURI",
			inputPublishAPICACert{
				manifests:      selfSignedCertSecret + certManagerCertSecret + customCertSecret,
				publishAPIMode: "Global",
				httpMode:       "OnlyInURI",
			},
			"discoveredKubernetesCA",
		),
		Entry("Without secret: Global: OnlyInURI",
			inputPublishAPICACert{
				manifests:      "",
				publishAPIMode: "Global",
				httpMode:       "OnlyInURI",
			},
			"discoveredKubernetesCA",
		),
		Entry("Without secret: Global: OnlyInURI with custom CA",
			inputPublishAPICACert{
				manifests:          "",
				publishAPIMode:     "Global",
				httpMode:           "OnlyInURI",
				kubeconfigMasterCA: pointer.String("testMasterCA"),
			},
			"testMasterCA",
		),
		Entry("Without secret: Global: OnlyInURI with custom CA empty",
			inputPublishAPICACert{
				manifests:          "",
				publishAPIMode:     "Global",
				httpMode:           "OnlyInURI",
				kubeconfigMasterCA: pointer.String(""),
			},
			"",
		),
	)

	Context("With all secrets in cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthn.publishAPI.https.mode", "SelfSigned")
			f.ValuesSet("userAuthn.https.mode", "CertManager")
			f.BindingContexts.Set(f.KubeStateSet(selfSignedCertSecret + certManagerCertSecret + customCertSecret))
			f.RunHook()
		})

		It("Should delete not matching secrets", func() {
			Expect(f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-selfsigned").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-user-authn", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})
})
