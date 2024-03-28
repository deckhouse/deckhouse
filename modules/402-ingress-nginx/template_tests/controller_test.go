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

package template_tests

import (
	"encoding/json"
	"fmt"
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
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.21.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.deckhouse.io/deckhouse/fe")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd", "operator-prometheus-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "1.1")

		hec.ValuesSet("ingressNginx.internal.admissionCertificate.ca", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.cert", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.key", "test")
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.namespaces", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.ingresses", json.RawMessage("[]"))
	})
	Context("With ingress nginx controller in values", func() {
		BeforeEach(func() {
			var certificates string
			for _, ingressName := range []string{"test", "test-lbwpp", "test-next", "solid"} {
				certificates += fmt.Sprintf(`
- controllerName: %s
  ingressClass: nginx
  data:
    cert: teststring
    key: teststring
`, ingressName)
			}
			hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLS", certificates)

			hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", `
- name: test
  spec:
    config:
      use-proxy-protocol: true
      load-balance: ewma
    ingressClass: nginx
    additionalLogFields:
      my-cookie: "$cookie_MY_COOKIE"
    validationEnabled: true
    controllerVersion: "1.1"
    inlet: LoadBalancer
    hsts: true
    hstsOptions:
      maxAge: "123456789123456789"
    resourcesRequests:
      mode: Static
      static:
        cpu: 100m
        memory: 200Mi
    loadBalancer:
      annotations:
        my: annotation
        second: true
      sourceRanges:
      - 1.1.1.1
      - 2.2.2.2
    maxReplicas: 6
    minReplicas: 2
- name: test-lbwpp
  spec:
    config:
      load-balance: ewma
    ingressClass: nginx
    controllerVersion: "1.1"
    inlet: LoadBalancerWithProxyProtocol
    resourcesRequests:
      mode: Static
    loadBalancerWithProxyProtocol:
      annotations:
        my: annotation
        second: true
      sourceRanges:
      - 1.1.1.1
      - 2.2.2.2
    maxReplicas: 6
    minReplicas: 2
    additionalHeaders:
      X-Foo: bar
- name: test-without-hpa
  spec:
    inlet: LoadBalancer
    ingressClass: nginx
    controllerVersion: "1.1"
    maxReplicas: 3
    minReplicas: 3
- name: test-next
  spec:
    ingressClass: test
    controllerVersion: "1.1"
    inlet: "HostPortWithProxyProtocol"
    geoIP2:
      maxmindLicenseKey: abc12345
      maxmindEditionIDs: ["GeoIPTest", "GeoIPTest2"]
    resourcesRequests:
      mode: Static
    hostPortWithProxyProtocol:
      httpPort: 80
      httpsPort: 443
- name: solid
  spec:
    ingressClass: solid
    controllerVersion: "1.1"
    inlet: "HostWithFailover"
    resourcesRequests:
      mode: VPA
      static: {}
      vpa:
        cpu:
          max: 200m
        memory:
          max: 200Mi
        mode: Auto
    defaultSSLCertificate:
      secretRef:
        name: custom-secret
        namespace: default
- name: wait-lb-non-default
  spec:
    inlet: "HostPort"
    waitLoadBalancerOnTerminating: 333
- name: wait-lb-zero
  spec:
    inlet: "HostPort"
    waitLoadBalancerOnTerminating: 0
- name: filter
  spec:
    inlet: "HostWithFailover"
    acceptRequestsFrom:
    - 67.34.56.23/32
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			testD := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`
cpu: 100m
ephemeral-storage: 150Mi
memory: 200Mi`))
			Expect(testD.Field("spec.template.spec.containers.0.args").Array()).To(ContainElement(ContainSubstring(`--shutdown-grace-period=120`)))
			Expect(testD.Field("spec.template.spec.containers.0.args").AsStringSlice()).NotTo(ContainElement(ContainSubstring("--default-ssl-certificate=")))
			// publish service for LB
			Expect(testD.Field("spec.template.spec.containers.0.args").AsStringSlice()).To(ContainElement(ContainSubstring("--publish-service=d8-ingress-nginx/test-load-balancer")))

			cm := hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-config")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.log-format-upstream").String()).To(ContainSubstring(`"my-cookie": "$cookie_MY_COOKIE"`))
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "proxy-test-failover-config").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-test-auth-tls").Exists()).To(BeTrue())

			fakeIng := hec.KubernetesResource("Ingress", "d8-ingress-nginx", "test-custom-headers-reload")
			Expect(fakeIng.Field("spec.rules.0.http.paths.0.path").String()).To(Equal("/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-load-balancer").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-admission").Exists()).To(BeTrue())
			Expect(hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-ingress-nginx-admission").Exists()).To(BeTrue())
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
			// publish service for LB
			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-lbwpp").Field("spec.template.spec.containers.0.args").AsStringSlice()).To(ContainElement(ContainSubstring("--publish-service=d8-ingress-nginx/test-lbwpp-load-balancer")))
			Expect(hec.KubernetesResource("Deployment", "d8-ingress-nginx", "hpa-scaler-test-lbwpp").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("HorizontalPodAutoscaler", "d8-ingress-nginx", "hpa-scaler-test-lbwpp").Exists()).To(BeTrue())

			// HPA for controller with maxReplicas == minReplicas should not exists
			Expect(hec.KubernetesResource("Deployment", "d8-ingress-nginx", "hpa-scaler-test-without-hpa").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("HorizontalPodAutoscaler", "d8-ingress-nginx", "hpa-scaler-test-without-hpa").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-without-hpa").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("PrometheusRule", "d8-monitoring", "prometheus-metrics-adapter-d8-ingress-nginx-cpu-utilization-for-hpa").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "proxy-test-lbwpp-failover-config").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-test-lbwpp-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-lbwpp-load-balancer").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-lbwpp-load-balancer").Field("metadata.annotations")).To(MatchJSON(`{"my":"annotation", "second": "true"}`))
			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-lbwpp-load-balancer").Field("spec.loadBalancerSourceRanges")).To(MatchJSON(`["1.1.1.1","2.2.2.2"]`))

			configMapData = hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-config").Field("data")
			fakeIng = hec.KubernetesResource("Ingress", "d8-ingress-nginx", "test-lbwpp-custom-headers-reload")
			Expect(fakeIng.Field("spec.rules.0.http.paths.0.path").String()).To(Equal("/d18475119d75d3c873bd30e53f4615ef66bf84d9ae1508df173dcc114cfecbb4"))
			// Use the Raw property to check is value quoted correctly
			Expect(configMapData.Get("use-proxy-protocol").Raw).To(Equal(`"true"`))
			Expect(configMapData.Get("body-size").Raw).To(Equal(`"64m"`))
			Expect(configMapData.Get("load-balance").Raw).To(Equal(`"ewma"`))

			testNextDaemonSet := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-next")
			Expect(testNextDaemonSet.Exists()).To(BeTrue())

			Expect(testNextDaemonSet.Field(`metadata.annotations.ingress-nginx-controller\.deckhouse\.io/controller-version`).String()).To(Equal(`1.1`))
			Expect(testNextDaemonSet.Field(`metadata.annotations.ingress-nginx-controller\.deckhouse\.io/inlet`).String()).To(Equal(`HostPortWithProxyProtocol`))
			Expect(testNextDaemonSet.Field("spec.template.spec.containers.0.args").Array()).To(ContainElement(ContainSubstring(`--shutdown-grace-period=60`)))
			// should not have --publish-service, inlet: HostPort
			Expect(testNextDaemonSet.Field("spec.template.spec.containers.0.args").AsStringSlice()).NotTo(ContainElement(ContainSubstring("--publish-service=")))

			var testNextArgs []string
			for _, result := range testNextDaemonSet.Field("spec.template.spec.containers.0.args").Array() {
				testNextArgs = append(testNextArgs, result.String())
			}

			Expect(testNextArgs).Should(ContainElement("--maxmind-license-key=abc12345"))
			Expect(testNextArgs).Should(ContainElement("--maxmind-edition-ids=GeoIPTest,GeoIPTest2"))

			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-next-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-next-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "proxy-test-next-failover-config").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-test-next-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-next-load-balancer").Exists()).ToNot(BeTrue())

			hpaTest := hec.KubernetesResource("HorizontalPodAutoscaler", "d8-ingress-nginx", "hpa-scaler-test")
			Expect(hpaTest.Exists()).To(BeTrue())
			Expect(hpaTest.Field("spec.maxReplicas").Int()).To(Equal(int64(6)))
			Expect(hpaTest.Field("spec.minReplicas").Int()).To(Equal(int64(2)))

			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test-next").
				Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`
cpu: 350m
ephemeral-storage: 150Mi
memory: 500Mi`))
			Expect(hec.KubernetesResource("VerticalPodAutoscaler", "d8-ingress-nginx", "controller-test-next").Field("spec.updatePolicy.updateMode").String()).To(Equal("Off"))

			mainDS := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-solid")
			Expect(mainDS.Exists()).To(BeTrue())
			Expect(mainDS.Field("spec.updateStrategy.type").String()).To(Equal("RollingUpdate"))
			Expect(mainDS.Field("spec.template.spec.hostNetwork").String()).To(Equal("true"))
			Expect(mainDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			Expect(mainDS.Field("spec.template.spec.containers.0.args").Array()).To(ContainElement(ContainSubstring(`--shutdown-grace-period=0`)))
			Expect(mainDS.Field("spec.template.spec.containers.0.args").AsStringSlice()).To(ContainElement("--default-ssl-certificate=default/custom-secret"))

			manVPA := hec.KubernetesResource("VerticalPodAutoscaler", "d8-ingress-nginx", "controller-solid")
			Expect(manVPA.Exists()).To(BeTrue())
			Expect(manVPA.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(manVPA.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
- containerName: controller
  minAllowed:
    cpu: 10m
    memory: 50Mi
  maxAllowed:
    cpu: 200m
    memory: 200Mi`))

			failoverDS := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-solid-failover")
			Expect(failoverDS.Exists()).To(BeTrue())
			Expect(failoverDS.Field("spec.updateStrategy.type").String()).To(Equal("RollingUpdate"))
			Expect(failoverDS.Field("spec.template.spec.hostNetwork").String()).To(Equal("false"))
			Expect(failoverDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirst"))

			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "proxy-solid-failover").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "solid-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "solid-failover-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "solid-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-solid-auth-tls").Exists()).To(BeTrue())

			proxyConfigMap := hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "proxy-solid-failover-config")
			Expect(proxyConfigMap.Exists()).To(BeTrue())
			Expect(proxyConfigMap.Field(`data.accept-requests-from\.conf`).String()).To(Equal(""))

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "controller-solid-failover").Exists()).To(BeTrue())

			waitLbNonDefaultDs := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-wait-lb-non-default")
			Expect(waitLbNonDefaultDs.Exists()).To(BeTrue())
			Expect(waitLbNonDefaultDs.Field("spec.template.spec.containers.0.args").Array()).To(ContainElement(ContainSubstring(`--shutdown-grace-period=333`)))

			waitLbZeroDs := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-wait-lb-zero")
			Expect(waitLbZeroDs.Exists()).To(BeTrue())
			Expect(waitLbZeroDs.Field("spec.template.spec.containers.0.args").Array()).To(ContainElement(ContainSubstring(`--shutdown-grace-period=0`)))

			Expect(hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-filter").Exists()).To(BeTrue())
			proxyFilterConfigMap := hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "proxy-filter-failover-config")
			Expect(proxyFilterConfigMap.Exists()).To(BeTrue())
			Expect(proxyFilterConfigMap.Field(`data.accept-requests-from\.conf`).String()).To(Equal(`allow 67.34.56.23/32;
deny all;`))
		})

		Context("Vertical pod autoscaler CRD is disabled", func() {
			BeforeEach(func() {
				hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
				hec.HelmRender()
			})

			It("should render controller", func() {
				testD := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-test")
				Expect(testD.Exists()).To(BeTrue())
			})
		})
	})
})
