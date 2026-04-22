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

const globalValues = `
  clusterConfiguration:
    apiVersion: deckhouse.io/v1
    cloud:
      prefix: sandbox
      provider: vSphere
    clusterDomain: cluster.local
    clusterType: Cloud
    defaultCRI: Containerd
    kind: ClusterConfiguration
    kubernetesVersion: "Automatic"
    podSubnetCIDR: 10.111.0.0/16
    podSubnetNodeCIDRPrefix: "24"
    serviceSubnetCIDR: 10.222.0.0/16
  modulesImages:
    digests:
      controlPlaneManager:
        controlPlaneManager131: sha256:abcdefgh
        controlPlaneManager132: sha256:abcdefgh
        controlPlaneManager133: sha256:abcdefgh
        controlPlaneManager134: sha256:abcdefgh
        controlPlaneManager135: sha256:abcdefgh
        etcd: sha256:abcdefgh
        etcdBackup: sha256:abcdefgh
        kubeApiserver131: sha256:abcdefgh
        kubeApiserver132: sha256:abcdefgh
        kubeApiserver133: sha256:abcdefgh
        kubeApiserver134: sha256:abcdefgh
        kubeApiserver135: sha256:abcdefgh
        kubeControllerManager131: sha256:abcdefgh
        kubeControllerManager132: sha256:abcdefgh
        kubeControllerManager133: sha256:abcdefgh
        kubeControllerManager134: sha256:abcdefgh
        kubeControllerManager135: sha256:abcdefgh
        kubeScheduler131: sha256:abcdefgh
        kubeScheduler132: sha256:abcdefgh
        kubeScheduler133: sha256:abcdefgh
        kubeScheduler134: sha256:abcdefgh
        kubeScheduler135: sha256:abcdefgh
        updateObserver: sha256:abcdefgh
  internal:
    modules:
      resourcesRequests:
        milliCpuControlPlane: 1024
        memoryControlPlane: 536870912
  modules:
    placement: {}
  discovery:
    d8SpecificNodeCountByRole:
      worker: 1
      master:
        __ConstantChoices__: "3"
    podSubnet: 10.0.1.0/16
    kubernetesVersion: 1.32.13
`
const publishAPIValues = (`
publishAPI:
  ingress:
    enabled: true
    addKubeconfigGeneratorEntry: true
    https:
      mode: SelfSigned
  loadBalancer:
    port: 443
`)

var _ = Describe("Module :: control-plane-manager :: helm template :: publish api", func() {
	hec := SetupHelmConfig(`controlPlaneManager: {}`)

	BeforeEach(func() {
		hec.ValuesSetFromYaml("global", globalValues)
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.32.13")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.ingressClass", "nginx")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		hec.ValuesSet("controlPlaneManager.internal.effectiveKubernetesVersion", "1.32")
		hec.ValuesSet("controlPlaneManager.internal.pkiChecksum", "4da1e937a9acd5475640d55cec899e77865e51ce2ab86d372c3ed1e19a532d19")
		hec.ValuesSet("controlPlaneManager.internal.rolloutEpoch", 2.049844452e+09)
		hec.ValuesSet("controlPlaneManager.internal.authn.enableBasicAuth", true)
		hec.ValuesSet("controlPlaneManager.internal.authn.publishedAPIKubeconfigGeneratorMasterCA", "publishedapica")
		hec.ValuesSet("controlPlaneManager.internal.selfSignedCA.cert", "test")
		hec.ValuesSet("controlPlaneManager.internal.selfSignedCA.key", "testCA")
		hec.ValuesSetFromYaml("controlPlaneManager.apiserver", publishAPIValues)
	})

	Context("By default", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})
		It("Should deploy publish api and kubeconfig generator", func() {
			certificate := hec.KubernetesResource("Certificate", "kube-system", "kubernetes-tls-selfsigned")
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("Issuer"))
			Expect(certificate.Field("spec.issuerRef.name").String()).To(Equal("kubernetes-api"))
			Expect(hec.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Service", "kube-system", "d8-control-plane-apiserver").Exists()).To(BeFalse())
		})
	})

	Context("With global mode CustomCertificate", func() {
		BeforeEach(func() {
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "Global")
			hec.ValuesSet("global.modules.https.mode", "CustomCertificate")
			hec.ValuesSetFromYaml("controlPlaneManager.internal.customCertificateData", `
tls.crt: CRTCRTCRT
tls.key: KEYKEYKEY
`)

			hec.HelmRender()
		})

		It("Should deploy secret certificate", func() {
			Expect(hec.KubernetesResource("Certificate", "kube-system", "kubernetes-tls-selfsigned").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Certificate", "kube-system", "kubernetes-tls").Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeTrue())
		})
	})

	Context("With publish API global mode", func() {
		BeforeEach(func() {
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "Global")
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.ingressClass", "my-ingress-class")
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.global.kubeconfigGeneratorMasterCA", "simplecastring")
			hec.HelmRender()
		})
		It("Should use cluster issuer", func() {
			certificate := hec.KubernetesResource("Certificate", "kube-system", "kubernetes-tls")
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("ClusterIssuer"))
			Expect(hec.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("With publish API global mode and route53 issuer", func() {
		BeforeEach(func() {
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.mode", "Global")
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.https.global.kubeconfigGeneratorMasterCA", "simplecastring")
			hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "route53")
			hec.HelmRender()
		})
		It("Should use cluster issuer and dns challenge", func() {
			certificate := hec.KubernetesResource("Certificate", "kube-system", "kubernetes-tls")
			Expect(certificate.Field("spec.issuerRef.kind").String()).To(Equal("ClusterIssuer"))
			Expect(certificate.Field("spec.issuerRef.name").String()).To(Equal("route53"))
			Expect(hec.KubernetesResource("Secret", "kube-system", "kubernetes-tls-customcertificate").Exists()).To(BeFalse())
		})
	})

	Context("With provider with enableBasicAuth option", func() {
		BeforeEach(func() {
			hec.ValuesSet("controlPlaneManager.internal.authn.enableBasicAuth", true)
			hec.ValuesSetFromYaml("controlPlaneManager.apiserver.publishAPI.ingress.whitelistSourceRanges", `
- 1.1.1.1
- 192.168.0.0/24
`)
			hec.HelmRender()
		})
		It("Should deploy basic auth ingress with rewrite", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("Ingress", "kube-system", "kubernetes-api").Field(
				"metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/configuration-snippet").String()).To(
				Equal("if ($http_authorization ~ \"^(.*)Basic(.*)$\") {\n  rewrite ^(.*)$ /basic-auth$1;\n}\nlocation ~ ^/(healthz|livez|readyz) {\n  deny all;\n  return 403;\n}\n"))
			Expect(hec.KubernetesResource("Ingress", "kube-system", "kubernetes-api").Field(
				"metadata.annotations.nginx\\.ingress\\.kubernetes\\.io/whitelist-source-range").String()).To(
				Equal("1.1.1.1,192.168.0.0/24"))
		})
	})

	Context("Default service loadBalancer", func() {
		BeforeEach(func() {
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.ingress.enabled", false)
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.loadBalancer.enabled", true)
			hec.HelmRender()
		})
		It("Should render service loadBalancer", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("Service", "kube-system", "d8-control-plane-apiserver").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "kube-system", "kubernetes").Exists()).To(BeFalse())

			service := hec.KubernetesResource("Service", "kube-system", "d8-control-plane-apiserver")
			port := int(service.Field("spec.ports").Array()[0].Map()["port"].Int())
			Expect(port).To(Equal(443))
		})
	})

	Context("Service loadBalancer with custom port, sourceRanges, annotation", func() {
		BeforeEach(func() {
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.loadBalancer.enabled", true)
			hec.ValuesSet("controlPlaneManager.apiserver.publishAPI.loadBalancer.port", 4343)
			hec.ValuesSetFromYaml("controlPlaneManager.apiserver.publishAPI.loadBalancer.sourceRanges", `
- 1.1.1.1/32
- 192.168.0.0/24
`)
			hec.ValuesSetFromYaml("controlPlaneManager.apiserver.publishAPI.loadBalancer.annotations", `
service.beta.kubernetes.io/aws-load-balancer-type: nlb
foo: bar
`)
			hec.HelmRender()
		})
		It("Should render custom settings for service loadBalancer", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesResource("Service", "kube-system", "d8-control-plane-apiserver").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "kube-system", "kubernetes").Exists()).To(BeTrue())

			service := hec.KubernetesResource("Service", "kube-system", "d8-control-plane-apiserver")

			port := int(service.Field("spec.ports").Array()[0].Map()["port"].Int())
			Expect(port).To(Equal(4343))

			sourceRanges := []string{}
			for _, r := range service.Field("spec.loadBalancerSourceRanges").Array() {
				sourceRanges = append(sourceRanges, r.String())
			}
			Expect(sourceRanges).To(ContainElements("1.1.1.1/32", "192.168.0.0/24"))

			Expect(service.Field("metadata.annotations.service\\.beta\\.kubernetes\\.io/aws-load-balancer-type").String()).To(Equal("nlb"))
			Expect(service.Field("metadata.annotations.foo").String()).To(Equal("bar"))
		})
	})
})
