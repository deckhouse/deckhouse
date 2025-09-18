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

package template_tests

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"

	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks"
)

var _ = Describe("Module :: user-authn :: helm template :: truncation", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.25.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
	})

	It("should render truncated name with labels and ingress backend references it", func() {
		longName := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbb"
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", `
			- name: `+longName+`
			encodedName: enc
			namespace: d8-test
			credentials:
				appDexSecret: dexSecret
				cookieSecret: cookieSecret
			spec:
				applications:
				- domain: app.example.com
				ingressClassName: test
			`)
		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		// Calculate expected truncated names
		baseFullName := fmt.Sprintf("%s-dex-authenticator", longName)
		truncatedBaseName, _, baseHash := hooks.SafeDNS1123Name(baseFullName)
		Expect(len(truncatedBaseName)).Should(BeNumerically("<=", 63))

		// Check Service
		svc := hec.KubernetesResource("Service", "d8-test", truncatedBaseName)
		Expect(svc.Exists()).To(BeTrue())
		Expect(svc.Field("metadata.labels.deckhouse\\.io/dex-authenticator-for").String()).To(Equal(longName))
		Expect(svc.Field("metadata.labels.deckhouse\\.io/name-truncated").String()).To(Equal("true"))
		Expect(svc.Field("metadata.labels.deckhouse\\.io/name-hash").String()).To(Equal(baseHash))

		// Deployment/PDB/VPA share the same name and labels
		dep := hec.KubernetesResource("Deployment", "d8-test", truncatedBaseName)
		Expect(dep.Exists()).To(BeTrue())
		Expect(dep.Field("metadata.labels.deckhouse\\.io/dex-authenticator-for").String()).To(Equal(longName))
		Expect(dep.Field("metadata.labels.deckhouse\\.io/name-hash").String()).To(Equal(baseHash))
		Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-test", truncatedBaseName).Exists()).To(BeTrue())
		Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-test", truncatedBaseName).Exists()).To(BeTrue())

		// Check Ingress
		ingressFullName := fmt.Sprintf("%s-dex-authenticator", longName)
		truncatedIngressName, _, ingressHash := hooks.SafeDNS1123Name(ingressFullName)
		ing := hec.KubernetesResource("Ingress", "d8-test", truncatedIngressName)
		Expect(ing.Exists()).To(BeTrue())
		Expect(ing.Field("metadata.labels.deckhouse\\.io/name-truncated").String()).To(Equal("true"))
		Expect(ing.Field("metadata.labels.deckhouse\\.io/name-hash").String()).To(Equal(ingressHash))
		Expect(ing.Field("spec.rules.0.http.paths.0.backend.service.name").String()).To(Equal(truncatedBaseName))

		// Check Secret
		secretFullName := fmt.Sprintf("dex-authenticator-%s", longName)
		truncatedSecretName, _, secretHash := hooks.SafeDNS1123Name(secretFullName)
		secret := hec.KubernetesResource("Secret", "d8-test", truncatedSecretName)
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field("metadata.labels.deckhouse\\.io/name-truncated").String()).To(Equal("true"))
		Expect(secret.Field("metadata.labels.deckhouse\\.io/name-hash").String()).To(Equal(secretHash))
	})
})
