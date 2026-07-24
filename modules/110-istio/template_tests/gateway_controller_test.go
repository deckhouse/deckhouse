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

var _ = Describe("Module :: istio :: helm template :: gateway controllers", func() {
	f := SetupHelmConfig(``)

	Context("ingress gateway controller with inlet NodePort is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: nodeport-test
  spec:
    ingressGatewayClass: np
    inlet: NodePort
    nodePort:
      httpPort: 30080
      httpsPort: 30443
    resourcesRequests:
      mode: VPA
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("serviceaccount", "d8-ingress-istio", "ingress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("role", "d8-ingress-istio", "istio:ingress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("rolebinding", "d8-ingress-istio", "istio:ingress-gateway-controller").Exists()).To(BeTrue())

			ingressVpa := f.KubernetesResource("verticalpodautoscaler", "d8-ingress-istio", "ingress-gateway-controller-nodeport-test")
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-nodeport-test")
			ingressSvc := f.KubernetesResource("service", "d8-ingress-istio", "ingress-gateway-controller-nodeport-test")
			Expect(ingressVpa.Exists()).To(BeTrue())
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressSvc.Exists()).To(BeTrue())

			Expect(ingressVpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(ingressVpa.Field("spec.resourcePolicy").String()).To(MatchJSON(`{"containerPolicies":[{"containerName":"istio-proxy","controlledValues":"RequestsAndLimits","maxAllowed":{"cpu":"1000m","memory":"2000Mi"},"minAllowed":{"cpu":"100m","memory":"128Mi"}}]}`))

			Expect(ingressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"ingress-gateway-controller","heritage":"deckhouse","instance":"nodeport-test","istio":"ingressgateway","istio.deckhouse.io/ingress-gateway-class":"np","istio.io/dataplane-mode":"none","module":"istio"}`))

			Expect(ingressSvc.Field("spec.type").String()).To(Equal("NodePort"))
		})
	})
	Context("ingress gateway controller with inlet LoadBalancer is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: loadbalancer-test
  spec:
    ingressGatewayClass: lb
    inlet: LoadBalancer
    loadBalancer:
      annotations:
        aaa: bbb
    resourcesRequests:
      mode: Static
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("serviceaccount", "d8-ingress-istio", "ingress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("role", "d8-ingress-istio", "istio:ingress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("rolebinding", "d8-ingress-istio", "istio:ingress-gateway-controller").Exists()).To(BeTrue())

			ingressVpa := f.KubernetesResource("verticalpodautoscaler", "d8-ingress-istio", "ingress-gateway-controller-loadbalancer-test")
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-loadbalancer-test")
			ingressSvc := f.KubernetesResource("service", "d8-ingress-istio", "ingress-gateway-controller-loadbalancer-test")
			// Static mode does not render a VPA.
			Expect(ingressVpa.Exists()).To(BeFalse())
			Expect(ingressDs.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchJSON(`{"cpu":"100m","memory":"128Mi","ephemeral-storage":"60Mi"}`))
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressSvc.Exists()).To(BeTrue())

			Expect(ingressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"ingress-gateway-controller","heritage":"deckhouse","instance":"loadbalancer-test","istio":"ingressgateway","istio.deckhouse.io/ingress-gateway-class":"lb","istio.io/dataplane-mode":"none","module":"istio"}`))

			Expect(ingressSvc.Field("metadata.annotations").String()).To(MatchJSON(`{ "aaa": "bbb" }`))
			Expect(ingressSvc.Field("spec.type").String()).To(Equal("LoadBalancer"))
			Expect(ingressSvc.Field("spec.loadBalancerClass").Exists()).To(BeFalse())
		})
	})
	Context("ingress gateway controller with inlet LoadBalancer and loadBalancerClass is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: loadbalancerclass-test
  spec:
    ingressGatewayClass: lbc
    inlet: LoadBalancer
    loadBalancer:
      loadBalancerClass: my-lb-class
    resourcesRequests:
      mode: Static
`)
			f.HelmRender()
		})

		It("should render the service with spec.loadBalancerClass", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ingressSvc := f.KubernetesResource("service", "d8-ingress-istio", "ingress-gateway-controller-loadbalancerclass-test")
			Expect(ingressSvc.Exists()).To(BeTrue())

			Expect(ingressSvc.Field("spec.type").String()).To(Equal("LoadBalancer"))
			Expect(ingressSvc.Field("spec.loadBalancerClass").String()).To(Equal("my-lb-class"))
		})
	})
	Context("ingress gateway controller with inlet HostPort is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: hostport-test
  spec:
    ingressGatewayClass: hp
    inlet: HostPort
    hostPort:
      httpPort: 80
      httpsPort: 443
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("serviceaccount", "d8-ingress-istio", "ingress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("role", "d8-ingress-istio", "istio:ingress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("rolebinding", "d8-ingress-istio", "istio:ingress-gateway-controller").Exists()).To(BeTrue())

			ingressVpa := f.KubernetesResource("verticalpodautoscaler", "d8-ingress-istio", "ingress-gateway-controller-hostport-test")
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-hostport-test")
			ingressSvc := f.KubernetesResource("service", "d8-ingress-istio", "ingress-gateway-controller-hostport-test")
			// Omitted resourcesRequests uses the built-in VPA defaults.
			Expect(ingressVpa.Exists()).To(BeTrue())
			Expect(ingressVpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(ingressDs.Field("spec.template.spec.containers.0.resources.requests").String()).To(MatchJSON(`{"cpu":"100m","memory":"128Mi","ephemeral-storage":"60Mi"}`))
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressSvc.Exists()).To(BeTrue())

			Expect(ingressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"ingress-gateway-controller","heritage":"deckhouse","instance":"hostport-test","istio":"ingressgateway","istio.deckhouse.io/ingress-gateway-class":"hp","istio.io/dataplane-mode":"none","module":"istio"}`))
			istioProxyContainer := ingressDs.Field("spec.template.spec.containers").Array()
			Expect(len(istioProxyContainer)).To(Equal(1))
			Expect((istioProxyContainer[0].Get("ports"))).To(MatchJSON(`[
{"containerPort":8080,"hostPort":80,"name":"http","protocol":"TCP"},
{"containerPort":8443,"hostPort":443,"name":"https","protocol":"TCP"},
{"containerPort":15020,"name":"metrics","protocol":"TCP"},
{"containerPort":15090,"name":"http-envoy-prom","protocol":"TCP"},
{"containerPort":15021,"name":"status-port","protocol":"TCP"},
{"containerPort":15012,"name":"tls-istiod","protocol":"TCP"}
]`))

			Expect(ingressSvc.Field("spec.type").String()).To(Equal("ClusterIP"))
		})
	})

	Context("ingress gateway controller with networkTopology.numTrustedProxies", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: xff-test
  spec:
    ingressGatewayClass: xff
    inlet: LoadBalancer
    networkTopology:
      numTrustedProxies: 2
`)
			f.HelmRender()
		})

		It("renders the proxy.istio.io/config annotation with numTrustedProxies", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-xff-test")
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressDs.Field("spec.template.metadata.annotations").Get(`proxy\.istio\.io/config`).String()).
				To(MatchJSON(`{"gatewayTopology":{"numTrustedProxies":2}}`))
		})
	})

	Context("ingress gateway controller with networkTopology.numTrustedProxies set to 0", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: xff-zero-test
  spec:
    ingressGatewayClass: xff0
    inlet: LoadBalancer
    networkTopology:
      numTrustedProxies: 0
`)
			f.HelmRender()
		})

		It("renders numTrustedProxies even when it is zero", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-xff-zero-test")
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressDs.Field("spec.template.metadata.annotations").Get(`proxy\.istio\.io/config`).String()).
				To(MatchJSON(`{"gatewayTopology":{"numTrustedProxies":0}}`))
		})
	})

	Context("ingress gateway controller with networkTopology.proxyProtocol", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: pp-test
  spec:
    ingressGatewayClass: pp
    inlet: LoadBalancer
    networkTopology:
      proxyProtocol: true
`)
			f.HelmRender()
		})

		It("renders the proxy.istio.io/config annotation with an empty proxyProtocol object", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-pp-test")
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressDs.Field("spec.template.metadata.annotations").Get(`proxy\.istio\.io/config`).String()).
				To(MatchJSON(`{"gatewayTopology":{"proxyProtocol":{}}}`))
		})
	})

	Context("ingress gateway controller with both network topology settings", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: combined-topology-test
  spec:
    ingressGatewayClass: combined
    inlet: LoadBalancer
    networkTopology:
      numTrustedProxies: 1
      proxyProtocol: true
`)
			f.HelmRender()
		})

		It("renders both gateway topology settings", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-combined-topology-test")
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressDs.Field("spec.template.metadata.annotations").Get(`proxy\.istio\.io/config`).String()).
				To(MatchJSON(`{"gatewayTopology":{"numTrustedProxies":1,"proxyProtocol":{}}}`))
		})
	})

	Context("ingress gateway controller without networkTopology", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: no-topology-test
  spec:
    ingressGatewayClass: nt
    inlet: LoadBalancer
`)
			f.HelmRender()
		})

		It("does not render the proxy.istio.io/config annotation", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-no-topology-test")
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressDs.Field("spec.template.metadata.annotations").Get(`proxy\.istio\.io/config`).Exists()).To(BeFalse())
		})

		It("uses the upstream gateway probes and metrics metadata", func() {
			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-no-topology-test")
			istioProxy := ingressDs.Field("spec.template.spec.containers.0")
			Expect(istioProxy.Get("ports").String()).
				To(ContainSubstring(`"containerPort":15020,"name":"metrics","protocol":"TCP"`))
			Expect(istioProxy.Get("env.#(name==ISTIO_META_POD_PORTS).value").String()).To(Equal("[]"))
			Expect(istioProxy.Get("startupProbe").String()).To(MatchJSON(`{
				"failureThreshold":30,
				"httpGet":{"path":"/healthz/ready","port":15021,"scheme":"HTTP"},
				"initialDelaySeconds":1,
				"periodSeconds":1,
				"successThreshold":1,
				"timeoutSeconds":1
			}`))
			Expect(istioProxy.Get("readinessProbe").String()).To(MatchJSON(`{
				"failureThreshold":4,
				"httpGet":{"path":"/healthz/ready","port":15021,"scheme":"HTTP"},
				"initialDelaySeconds":0,
				"periodSeconds":15,
				"successThreshold":1,
				"timeoutSeconds":1
			}`))

			podMonitor := f.KubernetesResource("podmonitor", "d8-monitoring", "istio-ingress-gateway-controller")
			Expect(podMonitor.Exists()).To(BeTrue())
			Expect(podMonitor.Field("spec.podMetricsEndpoints.0.relabelings.1.replacement").String()).To(Equal("${1}:15020"))
		})
	})

	Context("egress gateway controllers are enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.egressControllers", `
- name: default
  spec:
    egressGatewayClass: egress
    resourcesRequests:
      mode: VPA
- name: restricted
  spec:
    egressGatewayClass: restricted
    resourcesRequests:
      mode: Static
      static:
        cpu: "200m"
        memory: "256Mi"
`)
			f.HelmRender()
		})

		It("renders isolated ClusterIP egress gateways", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("serviceaccount", "d8-egress-istio", "egress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("role", "d8-egress-istio", "istio:egress-gateway-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("rolebinding", "d8-egress-istio", "istio:egress-gateway-controller").Exists()).To(BeTrue())

			egressDs := f.KubernetesResource("daemonset", "d8-egress-istio", "egress-gateway-controller-default")
			egressSvc := f.KubernetesResource("service", "d8-egress-istio", "egress-gateway-controller-default")
			egressVpa := f.KubernetesResource("verticalpodautoscaler", "d8-egress-istio", "egress-gateway-controller-default")
			staticVpa := f.KubernetesResource("verticalpodautoscaler", "d8-egress-istio", "egress-gateway-controller-restricted")
			Expect(egressDs.Exists()).To(BeTrue())
			Expect(egressSvc.Exists()).To(BeTrue())
			Expect(egressVpa.Exists()).To(BeTrue())
			Expect(staticVpa.Exists()).To(BeFalse())

			Expect(egressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"egress-gateway-controller","heritage":"deckhouse","instance":"default","istio":"egressgateway","istio.deckhouse.io/egress-gateway-class":"egress","istio.io/dataplane-mode":"none","module":"istio"}`))
			Expect(egressDs.Field("spec.template.metadata.annotations.proxy\\.istio\\.io/config").Exists()).To(BeFalse())
			Expect(egressDs.Field("spec.template.spec.containers.0.ports.#(hostPort>0)").Exists()).To(BeFalse())
			Expect(egressSvc.Field("spec.type").String()).To(Equal("ClusterIP"))
			Expect(egressSvc.Field("spec.externalTrafficPolicy").Exists()).To(BeFalse())
			Expect(egressSvc.Field("spec.ports.#(nodePort>0)").Exists()).To(BeFalse())
			Expect(egressSvc.Field("spec.selector.istio\\.deckhouse\\.io/egress-gateway-class").String()).To(Equal("egress"))

			podMonitor := f.KubernetesResource("podmonitor", "d8-monitoring", "istio-egress-gateway-controller")
			Expect(podMonitor.Exists()).To(BeTrue())
			Expect(podMonitor.Field("spec.namespaceSelector.matchNames.0").String()).To(Equal("d8-egress-istio"))
		})
	})

	Context("gateway controller VPA requests fall back when VPA is disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "cert-manager", "cni-cilium"]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.egressControllers", `
- name: egress-vpa-fallback
  spec:
    egressGatewayClass: egress-fallback
    resourcesRequests:
      mode: VPA
`)
			f.ValuesSetFromYaml("istio.internal.ingressControllers", `
- name: ingress-vpa-fallback
  spec:
    ingressGatewayClass: ingress-fallback
    inlet: LoadBalancer
    resourcesRequests:
      mode: VPA
`)
			f.HelmRender()
		})

		It("sets default CPU and memory requests for both gateways", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			egressDs := f.KubernetesResource("daemonset", "d8-egress-istio", "egress-gateway-controller-egress-vpa-fallback")
			Expect(egressDs.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("100m"))
			Expect(egressDs.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("128Mi"))

			ingressDs := f.KubernetesResource("daemonset", "d8-ingress-istio", "ingress-gateway-controller-ingress-vpa-fallback")
			Expect(ingressDs.Field("spec.template.spec.containers.0.resources.requests.cpu").String()).To(Equal("100m"))
			Expect(ingressDs.Field("spec.template.spec.containers.0.resources.requests.memory").String()).To(Equal("128Mi"))
		})
	})

})
