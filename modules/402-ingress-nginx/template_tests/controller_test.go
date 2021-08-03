/*
Copyright 2021 Flant CJSC

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
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.19.11")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "0.25")

		hec.ValuesSet("global.modulesImages.tags.descheduler.descheduler", "tag")
	})
	Context("With ingress nginx controller in values", func() {
		BeforeEach(func() {
			var certificates string
			for _, ingressName := range []string{"test", "test-lbwpp", "test-next", "solid"} {
				certificates += fmt.Sprintf(`
- controllerName: %s
  ingressClass: nginx
  data:
    certificate: teststring
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
    controllerVersion: "0.26"
    inlet: LoadBalancer
    nodeSelector:
      node-role.deckhouse.io/frontend: ""
      node-role.kubernetes.io/frontend: ""
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
    controllerVersion: "0.26"
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
- name: test-next
  spec:
    ingressClass: test
    controllerVersion: "0.33"
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
    controllerVersion: "0.33"
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
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			testD := hec.KubernetesResource("Deployment", "d8-ingress-nginx", "controller-test")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchYAML(`
cpu: 100m
ephemeral-storage: 150Mi
memory: 200Mi`))
			Expect(testD.Field("spec.template.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution").String()).To(MatchYAML(`
- weight: 100
  podAffinityTerm:
    labelSelector:
      matchExpressions:
      - key: app
        operator: In
        values:
        - controller
      - key: name
        operator: In
        values:
        - test
    topologyKey: kubernetes.io/hostname`))
			Expect(testD.Field("spec.template.spec.topologySpreadConstraints").String()).To(MatchYAML(`
- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: ScheduleAnyway
  labelSelector:
    matchExpressions:
    - key: app
      operator: In
      values:
      - controller
    - key: name
      operator: In
      values:
      - test
`))

			testDeschedulerConfigMap := hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "descheduler-config-test")
			Expect(testDeschedulerConfigMap.Exists()).To(BeTrue())
			testDeschedulerConfigMapData := testDeschedulerConfigMap.Field("data")
			Expect(testDeschedulerConfigMapData.Get("policy\\.yaml").String()).To(MatchYAML(`
apiVersion: "descheduler/v1alpha1"
kind: "DeschedulerPolicy"
nodeSelector: node-role.deckhouse.io/frontend=,node-role.kubernetes.io/frontend=
evictLocalStoragePods: true
evictSystemCriticalPods: true
strategies:
  "RemovePodsViolatingTopologySpreadConstraint":
    enabled: true
    params:
      includeSoftConstraints: true
      labelSelector:
        matchLabels:
          app: controller
          name: test
      namespaces:
        include:
          - "d8-ingress-nginx"`))

			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-test-auth-tls").Exists()).To(BeTrue())

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

			Expect(hec.KubernetesResource("Deployment", "d8-ingress-nginx", "controller-test-lbwpp").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-lbwpp-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-test-lbwpp-auth-tls").Exists()).To(BeTrue())

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

			Expect(testNextDaemonSet.Field(`metadata.annotations.ingress-nginx-controller\.deckhouse\.io/controller-version`).String()).To(Equal(`0.33`))
			Expect(testNextDaemonSet.Field(`metadata.annotations.ingress-nginx-controller\.deckhouse\.io/inlet`).String()).To(Equal(`HostPortWithProxyProtocol`))

			var testNextArgs []string
			for _, result := range testNextDaemonSet.Field("spec.template.spec.containers.0.args").Array() {
				testNextArgs = append(testNextArgs, result.String())
			}

			Expect(testNextArgs).Should(ContainElement("--maxmind-license-key=abc12345"))
			Expect(testNextArgs).Should(ContainElement("--maxmind-edition-ids=GeoIPTest,GeoIPTest2"))

			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-next-config").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-ingress-nginx", "test-next-custom-headers").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-ingress-nginx", "ingress-nginx-test-next-auth-tls").Exists()).To(BeTrue())

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "test-next-load-balancer").Exists()).ToNot(BeTrue())

			hpaTest := hec.KubernetesResource("HorizontalPodAutoscaler", "d8-ingress-nginx", "controller-test")
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
			Expect(mainDS.Field("spec.updateStrategy.type").String()).To(Equal("OnDelete"))
			Expect(mainDS.Field("spec.template.spec.hostNetwork").String()).To(Equal("true"))
			Expect(mainDS.Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))

			vpaSolid := hec.KubernetesResource("VerticalPodAutoscaler", "d8-ingress-nginx", "controller-solid")
			Expect(vpaSolid.Exists()).To(BeTrue())
			Expect(vpaSolid.Field("spec.updatePolicy.updateMode").String()).To(Equal("Auto"))
			Expect(vpaSolid.Field("spec.resourcePolicy.containerPolicies").String()).To(MatchYAML(`
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

			Expect(hec.KubernetesResource("Service", "d8-ingress-nginx", "controller-solid-failover").Exists()).To(BeTrue())
		})
	})
})
