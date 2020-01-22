package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: ingress-nginx :: helm template :: controllers ", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.clusterVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "0.25")
	})
	Context("With ingress nginx controller in values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLStest", `
certificate: teststring
key: teststring
`)
			hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllerCRDs", `
- name: test
  spec:
    config: {}
    ingressClass: nginx
    controllerVersion: "0.25"
    inlet: LoadBalancer
    loadBalancer: {}
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "test-ingress-nginx-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-load-balancer").Exists()).To(BeTrue())
		})
	})
})
