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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: controller", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("userAuthn.internal.selfSignedCA.key", "test")
		hec.HelmRender()
	})

	It("Should run the controller with leader election that does not depend on the HA mode", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller")
		Expect(deployment.Exists()).To(BeTrue())
		// The binary elects a leader unconditionally now: outside HA the Deployment keeps its default
		// rolling update, so two pods run during every version change. HA_MODE used to switch the
		// election on, and nothing reads it any more.
		Expect(deployment.Field("spec.template.spec.containers.0.env").String()).ToNot(ContainSubstring("HA_MODE"))
	})

	It("Should keep the lease rights in the namespaced Role, not in the ClusterRole", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		// The lease lives in d8-user-authn. Granting it cluster-wide would tie it to the
		// ClusterRoleBinding: a controller that loses its cluster-wide access would then also lose the
		// lease and exit after RenewDeadline, instead of staying NotReady until the access is back.
		clusterRole := hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:controller")
		Expect(clusterRole.Exists()).To(BeTrue())
		Expect(clusterRole.Field("rules").String()).ToNot(ContainSubstring("leases"))

		role := hec.KubernetesResource("Role", "d8-user-authn", "controller")
		Expect(role.Exists()).To(BeTrue())
		Expect(role.Field("rules").String()).To(ContainSubstring("coordination.k8s.io"))
		Expect(role.Field("rules").String()).To(ContainSubstring("leases"))

		roleBinding := hec.KubernetesResource("RoleBinding", "d8-user-authn", "controller")
		Expect(roleBinding.Exists()).To(BeTrue())
		Expect(roleBinding.Field("roleRef.name").String()).To(Equal("controller"))
		Expect(roleBinding.Field("subjects.0.name").String()).To(Equal("controller"))
	})

	It("Should give the readiness probe time for both readyz checks", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller")
		// readyz runs cache-sync and api-access in sequence, 2 s each, and does not stop at the first
		// failure; the probe must outlast both or the body never says which one failed.
		Expect(deployment.Field("spec.template.spec.containers.0.readinessProbe.timeoutSeconds").Int()).To(BeNumerically(">=", 5))
		Expect(deployment.Field("spec.template.spec.containers.0.readinessProbe.httpGet.path").String()).To(Equal("/readyz"))
	})
})
