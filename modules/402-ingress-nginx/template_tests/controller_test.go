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
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "0.25")
	})
	Context("With ingress nginx controller in values", func() {
		BeforeEach(func() {
			for _, ingressName := range []string{"test", "test-lbwpp", "test-next"} {
				hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLS"+ingressName, `
certificate: teststring
key: teststring
`)
			}

			hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", `
- name: test
  spec:
    config:
      use-proxy-protocol: true
      load-balance: ewma
    ingressClass: nginx
    controllerVersion: "0.26"
    inlet: LoadBalancer
    hsts: true
    hstsOptions:
      maxAge: "123456789123456789"
    geoIP2: {}
    resourcesRequests:
      mode: VPA
      static: {}
      vpa:
        cpu:
          max: 100m
        memory:
          max: 200Mi
        mode: Auto
    loadBalancer:
      annotations:
        my: annotation
        second: true
      sourceRanges:
      - 1.1.1.1
      - 2.2.2.2
- name: test-lbwpp
  spec:
    config:
      load-balance: ewma
    ingressClass: nginx
    controllerVersion: "0.26"
    inlet: LoadBalancerWithProxyProtocol
    hstsOptions: {}
    loadBalancer: {}
    geoIP2: {}
    resourcesRequests:
      mode: Static
      static: {}
      vpa:
        cpu: {}
        memory: {}
    loadBalancerWithProxyProtocol:
      annotations:
        my: annotation
        second: true
      sourceRanges:
      - 1.1.1.1
      - 2.2.2.2
- name: test-next
  spec:
    config: {}
    ingressClass: test
    controllerVersion: "0.33"
    inlet: "HostPortWithProxyProtocol"
    hstsOptions: {}
    loadBalancer: {}
    loadBalancerWithProxyProtocol: {}
    geoIP2:
      maxmindLicenseKey: 12345
      maxmindEditionIDs: ["GeoIPTest", "GeoIPTest2"]
    resourcesRequests:
      mode: Static
      static: {}
      vpa:
        cpu: {}
        memory: {}
    hostPortWithProxyProtocol:
      httpPort: 80
      httpsPort: 443
    hostPort: {}
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "test-ingress-nginx-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-load-balancer").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-load-balancer").Field("metadata.annotations")).To(MatchJSON(`{"my":"annotation", "second": "true"}`))
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-load-balancer").Field("spec.loadBalancerSourceRanges")).To(MatchJSON(`["1.1.1.1","2.2.2.2"]`))

			configMapData := hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-config").Field("data")

			// Use the Raw property to check is value quoted correctly
			Expect(configMapData.Get("use-proxy-protocol").Raw).To(Equal(`"true"`))
			Expect(configMapData.Get("hsts").Raw).To(Equal(`"true"`))
			Expect(configMapData.Get("hsts-max-age").Raw).To(Equal(`"123456789123456789"`))

			Expect(configMapData.Get("body-size").Raw).To(Equal(`"64m"`))
			Expect(configMapData.Get("load-balance").Raw).To(Equal(`"ewma"`))

			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-lbwpp").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "test-lbwpp-ingress-nginx-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-lbwpp-load-balancer").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-lbwpp-load-balancer").Field("metadata.annotations")).To(MatchJSON(`{"my":"annotation", "second": "true"}`))
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-lbwpp-load-balancer").Field("spec.loadBalancerSourceRanges")).To(MatchJSON(`["1.1.1.1","2.2.2.2"]`))

			configMapData = hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-config").Field("data")

			// Use the Raw property to check is value quoted correctly
			Expect(configMapData.Get("use-proxy-protocol").Raw).To(Equal(`"true"`))
			Expect(configMapData.Get("body-size").Raw).To(Equal(`"64m"`))
			Expect(configMapData.Get("load-balance").Raw).To(Equal(`"ewma"`))

			testNextDaemonSet := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-next")
			Expect(testNextDaemonSet.Exists()).To(BeTrue())

			var testNextArgs []string
			for _, result := range testNextDaemonSet.Field("spec.template.spec.containers.0.args").Array() {
				testNextArgs = append(testNextArgs, result.String())
			}

			Expect(testNextArgs).Should(ContainElement("--maxmind-license-key=12345"))
			Expect(testNextArgs).Should(ContainElement("--maxmind-edition-ids=GeoIPTest,GeoIPTest2"))

			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-next-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-next-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "test-next-ingress-nginx-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-next-load-balancer").Exists()).ToNot(BeTrue())

			vpaTest := hec.KubernetesResource("VerticalPodAutoscaler", "d8-ingress-nginx", "controller-test")
			Expect(vpaTest.Exists()).To(BeTrue())
			Expect(vpaTest.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(vpaTest.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: controller
  minAllowed:
    cpu: 10m
    memory: 50Mi
  maxAllowed:
    cpu: 100m
    memory: 200Mi`))
			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-next").
				Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`
cpu: 50m
memory: 200Mi`))
			Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-ingress-nginx", "controller-test-next").Field("spec.updatePolicy.updateMode").String()).To(Equal("Off"))
		})
	})
})
