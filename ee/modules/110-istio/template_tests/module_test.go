/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
highAvailability: true
enabledModules: ["operator-prometheus-crd","cert-manager","vertical-pod-autoscaler-crd"]
modules:
  publicDomainTemplate: "%s.example.com"
  placement:
    customTolerationKeys:
      - "test-1"
      - "test-2/master"
  https:
    certManager:
      clusterIssuerName: letsencrypt
    mode: CertManager
modulesImages:
  registry: registry.deckhouse.io/deckhouse/fe
  registryDockercfg: Y2ZnCg==
  tags:
    common:
      kubeRbacProxy: hashstring
    istio:
      apiProxy: hashstring
      metadataExporter: hashstring
      metadataDiscovery: hashstring
      kiali: hashstring
      operatorV1x8x0alpha1: ov180a1hashstring
      operatorV1x8x1: ov181hashstring
      pilotV1x8x0alpha1: piv180a1hashstring
      pilotV1x8x1: piv181hashstring
      proxyv2V1x8x0alpha1: prv180a1hashstring
      proxyv2V1x8x1: prv181hashstring
discovery:
  clusterControlPlaneIsHighlyAvailable: true
  d8SpecificNodeCountByRole:
    system: 3
  kubernetesVersion: "1.19.0"
  clusterDomain: my.domain
  clusterUUID: aa-bb-cc
`

const istioValues = `
    internal:
      applicationNamespaces: []
      globalRevision: v1x8x1
      operatorRevisionsToInstall:  []
      revisionsToInstall: []
      federations: []
      multiclusters: []
      remoteAuthnKeypair:
        priv: aaa
        pub: bbb
      ca:
        cert: mycert
        key: mykey
        root: myroot
        chain: mychain
    auth:
      externalAuthentication: {}
      password: qqq
    outboundTrafficPolicyMode: AllowAny
    tlsMode: "Off"
    sidecar:
      includeOutboundIPRanges: ["10.0.0.0/24"]
      excludeOutboundIPRanges: ["1.2.3.4/32"]
      excludeInboundPorts: ["1", "2"]
      excludeOutboundPorts: ["3", "4"]
    multicluster:
      enabled: false
    federation:
      enabled: false
    alliance:
      ingressGateway:
        inlet: LoadBalancer
        nodePort: {}
    tracing: {}
    proxyConfig: {}
    controlPlane:
      resourcesManagement:
        mode: VPA
        vpa:
          mode: Auto
          cpu:
            min: "50m"
            max: "2"
          memory:
            min: "256Mi"
            max: "2Gi"
`

var _ = Describe("Module :: istio :: helm template :: main", func() {
	f := SetupHelmConfig(``)

	Context("tlsMode = Off, no federations or multiclusters", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.tlsMode", "Off")
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			paDefault := f.KubernetesResource("PeerAuthentication", "d8-istio", "default")
			drDefault := f.KubernetesResource("DestinationRule", "d8-istio", "default")
			drApiserver := f.KubernetesResource("DestinationRule", "d8-istio", "kube-apiserver")

			Expect(paDefault.Exists()).To(BeTrue())
			Expect(paDefault.Field("spec.mtls.mode").String()).To(Equal(`PERMISSIVE`))

			Expect(drDefault.Exists()).To(BeFalse())
			Expect(drApiserver.Exists()).To(BeFalse())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
		})
	})

	Context("tlsMode = Mutual, no federations or multiclusters", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.tlsMode", "Mutual")
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			paDefault := f.KubernetesResource("PeerAuthentication", "d8-istio", "default")
			drDefault := f.KubernetesResource("DestinationRule", "d8-istio", "default")
			drApiserver := f.KubernetesResource("DestinationRule", "d8-istio", "kube-apiserver")

			Expect(paDefault.Exists()).To(BeTrue())
			Expect(paDefault.Field("spec.mtls.mode").String()).To(Equal(`STRICT`))

			Expect(drDefault.Exists()).To(BeTrue())
			Expect(drDefault.Field("spec.host").String()).To(Equal(`*.my.domain`))

			Expect(drApiserver.Exists()).To(BeTrue())
			Expect(drApiserver.Field("spec.host").String()).To(Equal(`kubernetes.default.svc.my.domain`))

			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
		})
	})

	Context("tlsMode = MutualPermissive, no federations or multiclusters", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSet("istio.tlsMode", "MutualPermissive")
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			paDefault := f.KubernetesResource("PeerAuthentication", "d8-istio", "default")
			drDefault := f.KubernetesResource("DestinationRule", "d8-istio", "default")
			drApiserver := f.KubernetesResource("DestinationRule", "d8-istio", "kube-apiserver")

			Expect(paDefault.Exists()).To(BeTrue())
			Expect(paDefault.Field("spec.mtls.mode").String()).To(Equal(`PERMISSIVE`))

			Expect(drDefault.Exists()).To(BeTrue())
			Expect(drDefault.Field("spec.host").String()).To(Equal(`*.my.domain`))

			Expect(drApiserver.Exists()).To(BeTrue())
			Expect(drApiserver.Field("spec.host").String()).To(Equal(`kubernetes.default.svc.my.domain`))

			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
		})
	})

	Context("There are revisions to install, no federations or multiclusters", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1,v1x8x0alpha1]`)
			f.ValuesSetFromYaml("istio.internal.operatorRevisionsToInstall", `[v1x8x1,v1x8x0alpha1]`)
			f.ValuesSetFromYaml("istio.internal.applicationNamespaces", `[foo,bar]`)
			f.ValuesSet("istio.tlsMode", "Off")
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			iopV180alpha1 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x0alpha1")

			deploymentOperatorv181 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x8x1")
			deploymentOperatorv180alpha1 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x8x0alpha1")

			secretD8RegistryFoo := f.KubernetesResource("Secret", "foo", "d8-istio-sidecar-registry")
			secretD8RegistryBar := f.KubernetesResource("Secret", "bar", "d8-istio-sidecar-registry")

			secretCacerts := f.KubernetesResource("Secret", "d8-istio", "cacerts")

			serviceGlobal := f.KubernetesResource("Service", "d8-istio", "istiod")
			mwh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-istio-sidecar-injector-global")

			Expect(iopV181.Exists()).To(BeTrue())
			Expect(iopV180alpha1.Exists()).To(BeTrue())
			Expect(deploymentOperatorv181.Exists()).To(BeTrue())
			Expect(deploymentOperatorv180alpha1.Exists()).To(BeTrue())
			Expect(secretCacerts.Exists()).To(BeTrue())

			Expect(secretD8RegistryFoo.Exists()).To(BeTrue())
			Expect(secretD8RegistryBar.Exists()).To(BeTrue())

			Expect(mwh.Exists()).To(BeTrue())
			Expect(serviceGlobal.Exists()).To(BeTrue())

			Expect(iopV181.Field("spec.revision").String()).To(Equal(`v1x8x1`))
			Expect(iopV180alpha1.Field("spec.revision").String()).To(Equal(`v1x8x0alpha1`))

			Expect(iopV181.Field("spec.meshConfig.rootNamespace").String()).To(Equal(`d8-istio`))
			Expect(iopV180alpha1.Field("spec.meshConfig.rootNamespace").String()).To(Equal(`d8-istio`))

			Expect(deploymentOperatorv181.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.deckhouse.io/deckhouse/fe:ov181hashstring`))
			Expect(deploymentOperatorv180alpha1.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.deckhouse.io/deckhouse/fe:ov180a1hashstring`))

			Expect(iopV181.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.deckhouse.io/deckhouse/fe:prv181hashstring`))
			Expect(iopV180alpha1.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.deckhouse.io/deckhouse/fe:prv180a1hashstring`))

			Expect(iopV181.Field("spec.values.pilot.image").String()).To(Equal(`registry.deckhouse.io/deckhouse/fe:piv181hashstring`))
			Expect(iopV180alpha1.Field("spec.values.pilot.image").String()).To(Equal(`registry.deckhouse.io/deckhouse/fe:piv180a1hashstring`))

			Expect(mwh.Field("webhooks.0.clientConfig.service.name").String()).To(Equal(`istiod-v1x8x1`))
			Expect(mwh.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(`bXljZXJ0`)) // b64("mycert")
			Expect(serviceGlobal.Field("spec.selector").String()).To(MatchJSON(`{"app":"istiod","istio.io/rev":"v1x8x1"}`))

			Expect(secretCacerts.Field("data").String()).To(MatchJSON(`
				{
					"ca-cert.pem":"bXljZXJ0",
					"ca-key.pem":"bXlrZXk=",
					"cert-chain.pem":"bXljaGFpbg==",
					"root-cert.pem":"bXlyb290"
				}
`))

			Expect(iopV181.Field("spec.meshConfig.caCertificates").Exists()).To(BeFalse())
			Expect(iopV181.Field("spec.values.meshNetworks").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:alliance:metadata-exporter").Exists()).To(BeFalse())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "alliance:ingressgateway").Exists()).To(BeFalse())
		})
	})

	Context("There are some federations", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1]`)
			f.ValuesSetFromYaml("istio.internal.operatorRevisionsToInstall", `[v1x8x1]`)
			f.ValuesSet("istio.federation.enabled", true)
			f.ValuesSet("istio.tlsMode", "Off")
			f.ValuesSetFromYaml("istio.internal.federations", `
- name: neighbour-0
  trustDomain: n.n0
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
  ingressGateways:
  - address: 1.1.1.1
    port: 123
  publicServices:
  - hostname: xxx.yyy
    virtualIP: 2.2.2.2
    ports:
    - name: aaa
      port: 456
`)
			f.ValuesSetFromYaml("istio.internal.remotePublicMetadata", `
neighbour-0:
  clusterUUID: r-e-m-o-t-e
  rootCA: ---ROOT CA---
`)
			f.HelmRender()
		})

		It("ServiceEntry and DestinationRule must be created, metadata-exporter and ingressgateway must be deployed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			se := f.KubernetesResource("ServiceEntry", "d8-istio", "neighbour-0-xxx-yyy")
			dr := f.KubernetesResource("DestinationRule", "d8-istio", "neighbour-0-xxx-yyy")

			Expect(se.Exists()).To(BeTrue())
			Expect(dr.Exists()).To(BeTrue())

			Expect(se.Field("spec.hosts.0").String()).To(Equal("xxx.yyy"))
			Expect(se.Field("spec.ports").String()).To(MatchYAML(`
            - name: aaa
              number: 456
            `))
			Expect(se.Field("spec.endpoints").String()).To(MatchYAML(`
            - address: 1.1.1.1
              ports:
                aaa: 123
            `))
			Expect(se.Field("spec.addresses").String()).To(MatchYAML(`
            - 2.2.2.2
            `))

			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:alliance:metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:alliance:metadata-exporter").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Role", "d8-istio", "alliance:ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "alliance:ingressgateway").Exists()).To(BeTrue())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			Expect(iopV181.Field("spec.meshConfig.caCertificates").String()).To(MatchJSON(`[{"pem": "---ROOT CA---"}]`))
			Expect(iopV181.Field("spec.values.meshNetworks").Exists()).To(BeFalse())
		})
	})

	Context("There are some multiclusters, multiclustersNeedIngressGateway = true", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1]`)
			f.ValuesSetFromYaml("istio.internal.operatorRevisionsToInstall", `[v1x8x1]`)
			f.ValuesSet("istio.multicluster.enabled", true)
			f.ValuesSet("istio.tlsMode", "Off")
			f.ValuesSet("istio.internal.multiclustersNeedIngressGateway", true)
			f.ValuesSetFromYaml("istio.internal.multiclusters", `
- name: neighbour-0
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
  enableIngressGateway: true
  apiHost: remote.api.example.com
  networkName: a-b-c-1-2-3
  apiJWT: aAaA.bBbB.CcCc
  ingressGateways:
  - address: 1.1.1.1
    port: 123
`)
			f.ValuesSetFromYaml("istio.internal.remotePublicMetadata", `
neighbour-0:
  clusterUUID: r-e-m-o-t-e
  rootCA: ---ROOT CA---
`)
			f.HelmRender()
		})

		It("ServiceEntry and DestinationRule must be created, metadata-exporter and ingressgateway must be deployed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			kubeconfigSecret := f.KubernetesResource("Secret", "d8-istio", "istio-remote-secret-neighbour-0")
			Expect(kubeconfigSecret.Exists()).To(BeTrue())
			Expect(kubeconfigSecret.Field("data.neighbour-0").Exists()).To(BeTrue())
			renderedKubeconfig, _ := base64.StdEncoding.DecodeString(kubeconfigSecret.Field("data.neighbour-0").String())
			Expect(renderedKubeconfig).To(MatchYAML(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://remote.api.example.com
  name: neighbour-0
contexts:
- context:
    cluster: neighbour-0
    user: neighbour-0
  name: neighbour-0
current-context: neighbour-0
preferences: {}
users:
- name: neighbour-0
  user:
    token: aAaA.bBbB.CcCc
`))

			Expect(f.KubernetesResource("Deployment", "d8-istio", "api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-istio", "api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "multicluster-api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:multicluster:api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:multicluster:api-proxy").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("Deployment", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Ingress", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-istio", "metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:istio:alliance:metadata-exporter").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:istio:alliance:metadata-exporter").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Gateway", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-istio", "ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "alliance-ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Role", "d8-istio", "alliance:ingressgateway").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("RoleBinding", "d8-istio", "alliance:ingressgateway").Exists()).To(BeTrue())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			Expect(iopV181.Field("spec.meshConfig.caCertificates").String()).To(MatchJSON(`[{"pem": "---ROOT CA---"}]`))
			Expect(iopV181.Field("spec.values.global.meshNetworks").String()).To(MatchYAML(`
a-b-c-1-2-3:
  endpoints:
  - fromRegistry: neighbour-0
  gateways:
  - address: 1.1.1.1
    port: 123
`))
		})
	})

	Context("istiod with default resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			Expect(iopV181.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
requests:
  cpu: 50m
  memory: 256Mi
  ephemeral-storage: 50Mi
limits: {}
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x8x1")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x8x1
updatePolicy:
  updateMode: Auto
resourcePolicy:
  containerPolicies:
  - containerName: discovery
    maxAllowed:
      cpu: "2"
      memory: "2Gi"
    minAllowed:
      cpu: "50m"
      memory: "256Mi"
    controlledValues: RequestsAndLimits
`))
		})
	})

	Context("istiod with custom static resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1]`)
			f.ValuesSetFromYaml("istio.controlPlane.resourcesManagement", `
mode: Static
static:
  requests:
    cpu: 11m
    memory: 22Mi
  limits:
    cpu: 33
    memory: 44Gi
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			Expect(iopV181.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
requests:
  cpu: 11m
  memory: 22Mi
  ephemeral-storage: 50Mi
limits:
  cpu: 33
  memory: 44Gi
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x8x1")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x8x1
updatePolicy:
  updateMode: "Off"
`))
		})
	})

	Context("istiod with custom vpa resourcesManagement configuration case #1", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1]`)
			f.ValuesSetFromYaml("istio.controlPlane.resourcesManagement", `
mode: VPA
vpa:
  mode: Initial
  cpu:
    min: 101m
    max: 1
    limitRatio: 2.5
  memory:
    min: 512Mi
    max: 5Gi
    limitRatio: 2.5
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			Expect(iopV181.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
limits:
  cpu: 253m
  memory: "1342177280"
requests:
  ephemeral-storage: 50Mi
  cpu: 101m
  memory: 512Mi
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x8x1")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x8x1
resourcePolicy:
  containerPolicies:
  - containerName: discovery
    controlledValues: RequestsAndLimits
    maxAllowed:
      cpu: "1"
      memory: 5Gi
    minAllowed:
      cpu: 101m
      memory: 512Mi
updatePolicy:
  updateMode: Initial
`))
		})
	})

	Context("istiod with custom vpa resourcesManagement configuration case #2", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1]`)
			f.ValuesSetFromYaml("istio.controlPlane.resourcesManagement", `
mode: VPA
vpa:
  mode: Initial
  cpu:
    min: 3
    max: 5
    limitRatio: 2.5
  memory:
    min: "333"
    max: 7Gi
    limitRatio: 2.5
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			Expect(iopV181.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
limits:
  cpu: 7500m
  memory: "833"
requests:
  ephemeral-storage: 50Mi
  cpu: "3"
  memory: "333"
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x8x1")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x8x1
resourcePolicy:
  containerPolicies:
  - containerName: discovery
    controlledValues: RequestsAndLimits
    maxAllowed:
      cpu: "5"
      memory: 7Gi
    minAllowed:
      cpu: "3"
      memory: "333"
updatePolicy:
  updateMode: Initial
`))
		})
	})
})
