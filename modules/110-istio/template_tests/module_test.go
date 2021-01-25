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

const globalValues = `
highAvailability: true
enabledModules: ["operator-prometheus-crd","cert-manager"]
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
  registry: registry.flant.com
  registryDockercfg: mydockercfg
  tags:
    common:
      kubeRbacProxy: hashstring
    istio:
      kiali: kialihashstring
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
  kubernetesVersion: 1.19
  clusterDomain: my.domain
`

const istioValues = `
    clusterName: mycluster
    internal:
      applicationNamespaces: []
      globalRevision: v1x8x1
      operatorRevisionsToInstall:  []
      revisionsToInstall: []
      ca:
        cert: mycert
        key: mykey
        root: myroot
        chain: mychain
    network: mynetwork
    sidecar:
      includeOutboundIPRanges: ["*"]
      excludeOutboundIPRanges: ["1.2.3.4"]
      excludeInboundPorts: ["1", "2"]
      excludeOutboundPorts: ["3", "4"]
`

var _ = Describe("Module :: istio :: helm template :: main", func() {
	f := SetupHelmConfig(``)

	Context("tlsMode = Off", func() {
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
		})
	})

	Context("tlsMode = Mutual", func() {
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
		})
	})

	Context("tlsMode = MutualPermissive", func() {
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
		})
	})

	Context("There are revisions to install", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.revisionsToInstall", `[v1x8x1,v1x8x0alpha1]`)
			f.ValuesSetFromYaml("istio.internal.operatorRevisionsToInstall", `[v1x8x1,v1x8x0alpha1]`)
			f.ValuesSetFromYaml("istio.internal.applicationNamespaces", `[foo,bar]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV181 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x1")
			iopV180alpha1 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x8x0alpha1")

			deploymentOperatorv181 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x8x1")
			deploymentOperatorv180alpha1 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x8x0alpha1")

			secretD8RegistryFoo := f.KubernetesResource("Secret", "foo", "deckhouse-registry")
			secretD8RegistryBar := f.KubernetesResource("Secret", "bar", "deckhouse-registry")

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

			Expect(deploymentOperatorv181.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.flant.com/istio/operator-v1x8x1:ov181hashstring`))
			Expect(deploymentOperatorv180alpha1.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.flant.com/istio/operator-v1x8x0alpha1:ov180a1hashstring`))

			Expect(iopV181.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.flant.com/istio/proxyv2-v1x8x1:prv181hashstring`))
			Expect(iopV180alpha1.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.flant.com/istio/proxyv2-v1x8x0alpha1:prv180a1hashstring`))

			Expect(iopV181.Field("spec.values.pilot.image").String()).To(Equal(`registry.flant.com/istio/pilot-v1x8x1:piv181hashstring`))
			Expect(iopV180alpha1.Field("spec.values.pilot.image").String()).To(Equal(`registry.flant.com/istio/pilot-v1x8x0alpha1:piv180a1hashstring`))

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
		})
	})
})
