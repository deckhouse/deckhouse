/*
Copyright 2023 Flant JSC

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
	"encoding/base64"
	"os"
	"path/filepath"
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
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterType: Cloud
  kubernetesVersion: "Automatic"
  podSubnetCIDR: "10.111.0.0/16"
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: "10.222.0.0/16"
  cloud:
    provider: OpenStack
highAvailability: true
enabledModules: ["operator-prometheus-crd","cert-manager","vertical-pod-autoscaler-crd","cni-cilium"]
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
      globalVersion: "1.13.7"
      versionMap:
        "1.13.7":
          revision: "v1x13x7"
          fullVersion: "1.13.7.0"
          imageSuffix: "V1x13x7"
        "1.12.6":
          revision: "v1x12x6"
          fullVersion: "1.12.6.1"
          imageSuffix: "V1x12x6"
      operatorVersionsToInstall:  []
      versionsToInstall: []
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
        password: qqq
    auth:
      externalAuthentication: {}
    outboundTrafficPolicyMode: AllowAny
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
      replicasManagement:
        mode: Standard
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
    dataPlane:
      trafficRedirectionSetupMode: CNIPlugin
`

func getSubdirs(dir string) ([]string, error) {
	var subdirs []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != dir && filepath.Base(path) == info.Name() {
			subdirs = append(subdirs, info.Name())
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return subdirs, nil
}

const (
	istioEETempaltesPath = "/deckhouse/ee/modules/110-istio/templates/"
	istioCETempaltesPath = "/deckhouse/modules/110-istio/templates/"
)

var _ = Describe("Module :: istio :: helm template :: main", func() {

	BeforeSuite(func() {
		subDirs, err := getSubdirs(istioEETempaltesPath)
		Expect(err).ShouldNot(HaveOccurred())
		for _, subDir := range subDirs {
			err := os.Symlink(istioEETempaltesPath+subDir, istioCETempaltesPath+subDir)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	AfterSuite(func() {
		subDirs, err := getSubdirs(istioEETempaltesPath)
		Expect(err).ShouldNot(HaveOccurred())
		for _, subDir := range subDirs {
			err := os.Remove(istioCETempaltesPath + subDir)
			Expect(err).ShouldNot(HaveOccurred())
		}
	})

	f := SetupHelmConfig(``)

	Context("no federations or multiclusters", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			mwh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-istio-sidecar-injector-global")
			Expect(mwh.Exists()).To(BeTrue())
			Expect(len(mwh.Field("webhooks").Array())).To(Equal(2))

			drApiserver := f.KubernetesResource("DestinationRule", "d8-istio", "kube-apiserver")

			Expect(drApiserver.Exists()).To(BeTrue())
			Expect(drApiserver.Field("spec.host").String()).To(Equal(`kubernetes.default.svc.my.domain`))
			Expect(drApiserver.Field("spec.trafficPolicy.tls.mode").String()).To(Equal(`DISABLE`))

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
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ingressgateway").Exists()).To(BeFalse())
		})
	})

	Context("There are revisions to install, no federations or multiclusters", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7","1.12.6"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.13.7","1.12.6"]`)
			f.ValuesSetFromYaml("istio.internal.applicationNamespaces", `[foo,bar]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			mwh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-istio-sidecar-injector-global")
			Expect(mwh.Exists()).To(BeTrue())
			Expect(len(mwh.Field("webhooks").Array())).To(Equal(2))

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			iopV12 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x12x6")

			deploymentOperatorV13 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x13x7")
			deploymentOperatorV12 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x12x6")

			secretD8RegistryFoo := f.KubernetesResource("Secret", "foo", "d8-istio-sidecar-registry")
			secretD8RegistryBar := f.KubernetesResource("Secret", "bar", "d8-istio-sidecar-registry")

			secretCacerts := f.KubernetesResource("Secret", "d8-istio", "cacerts")

			serviceGlobal := f.KubernetesResource("Service", "d8-istio", "istiod")

			Expect(iopV13.Exists()).To(BeTrue())
			Expect(iopV12.Exists()).To(BeTrue())
			Expect(deploymentOperatorV13.Exists()).To(BeTrue())
			Expect(deploymentOperatorV12.Exists()).To(BeTrue())
			Expect(secretCacerts.Exists()).To(BeTrue())

			Expect(secretD8RegistryFoo.Exists()).To(BeTrue())
			Expect(secretD8RegistryBar.Exists()).To(BeTrue())

			Expect(mwh.Exists()).To(BeTrue())
			Expect(serviceGlobal.Exists()).To(BeTrue())

			Expect(iopV13.Field("spec.revision").String()).To(Equal(`v1x13x7`))
			Expect(iopV12.Field("spec.revision").String()).To(Equal(`v1x12x6`))

			Expect(iopV13.Field("spec.meshConfig.rootNamespace").String()).To(Equal(`d8-istio`))
			Expect(iopV12.Field("spec.meshConfig.rootNamespace").String()).To(Equal(`d8-istio`))

			Expect(deploymentOperatorV13.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.example.com@imageHash-istio-operatorV1x13x7`))
			Expect(deploymentOperatorV12.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.example.com@imageHash-istio-operatorV1x12x6`))

			Expect(iopV13.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.example.com@imageHash-istio-proxyv2V1x13x7`))
			Expect(iopV12.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.example.com@imageHash-istio-proxyv2V1x12x6`))

			Expect(iopV13.Field("spec.values.pilot.image").String()).To(Equal(`registry.example.com@imageHash-istio-pilotV1x13x7`))
			Expect(iopV12.Field("spec.values.pilot.image").String()).To(Equal(`registry.example.com@imageHash-istio-pilotV1x12x6`))

			Expect(mwh.Field("webhooks.0.clientConfig.service.name").String()).To(Equal(`istiod-v1x13x7`))
			Expect(mwh.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(`bXljZXJ0`)) // b64("mycert")
			Expect(serviceGlobal.Field("spec.selector").String()).To(MatchJSON(`{"app":"istiod","istio.io/rev":"v1x13x7"}`))

			Expect(secretCacerts.Field("data").String()).To(MatchJSON(`
				{
					"ca-cert.pem":"bXljZXJ0",
					"ca-key.pem":"bXlrZXk=",
					"cert-chain.pem":"bXljaGFpbg==",
					"root-cert.pem":"bXlyb290"
				}
`))

			Expect(iopV13.Field("spec.meshConfig.caCertificates").Exists()).To(BeFalse())
			Expect(iopV13.Field("spec.values.meshNetworks").Exists()).To(BeFalse())

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
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ingressgateway").Exists()).To(BeFalse())
		})
	})

	Context("There are some federations", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.13.7"]`)
			f.ValuesSet("istio.federation.enabled", true)
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

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.meshConfig.caCertificates").String()).To(MatchJSON(`[{"pem": "---ROOT CA---"}]`))
			Expect(iopV13.Field("spec.values.meshNetworks").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ingressgateway").Exists()).To(BeTrue())
		})
	})

	Context("Cloud provider OpenStack", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.13.7"]`)
			f.HelmRender()
		})
		It("CLOUD_PROVIDER env should be 'none'", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Exists()).To(BeTrue())
			Expect(iopV13.Field("spec.meshConfig.defaultConfig.proxyMetadata.CLOUD_PLATFORM").String()).To(Equal("none"))
		})
	})

	Context("Cloud provider AWS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.clusterConfiguration.cloud.provider", "AWS")
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.13.7"]`)
			f.HelmRender()
		})
		It("CLOUD_PROVIDER env should be 'aws'", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Exists()).To(BeTrue())
			Expect(iopV13.Field("spec.meshConfig.defaultConfig.proxyMetadata.CLOUD_PLATFORM").String()).To(Equal("aws"))
		})
	})

	Context("There are some multiclusters, multiclustersNeedIngressGateway = true", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.13.7"]`)
			f.ValuesSet("istio.multicluster.enabled", true)
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

			mwh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-istio-sidecar-injector-global")
			Expect(mwh.Exists()).To(BeTrue())
			Expect(len(mwh.Field("webhooks").Array())).To(Equal(2))

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

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.meshConfig.caCertificates").String()).To(MatchJSON(`[{"pem": "---ROOT CA---"}]`))
			Expect(iopV13.Field("spec.values.global.meshNetworks").String()).To(MatchYAML(`
a-b-c-1-2-3:
  endpoints:
  - fromRegistry: neighbour-0
  gateways:
  - address: 1.1.1.1
    port: 123
`))
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ingressgateway").Exists()).To(BeTrue())
		})
	})

	Context("istiod with default resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
requests:
  cpu: 50m
  memory: 256Mi
  ephemeral-storage: 50Mi
limits: {}
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x13x7")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x13x7
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

	Context("istiod with controlPlane custom static resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
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

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
requests:
  cpu: 11m
  memory: 22Mi
  ephemeral-storage: 50Mi
limits:
  cpu: 33
  memory: 44Gi
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x13x7")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x13x7
updatePolicy:
  updateMode: "Off"
`))
		})
	})

	Context("istiod with sidecar not use custom static resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.values.global.proxy.resources").String()).To(MatchYAML(`{}`))
		})
	})

	Context("istiod with sidecar custom static resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
			f.ValuesSetFromYaml("istio.sidecar.resourcesManagement", `
mode: Static
static:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    memory: 2Gi
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.values.global.proxy.resources").String()).To(MatchYAML(`
requests:
  cpu: 200m
  memory: 256Mi
limits:
  memory: 2Gi
`))
		})
	})

	Context("istiod with custom vpa resourcesManagement configuration case #1", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
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

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
limits:
  cpu: 253m
  memory: "1342177280"
requests:
  ephemeral-storage: 50Mi
  cpu: 101m
  memory: 512Mi
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x13x7")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x13x7
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
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.13.7"]`)
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

			iopV13 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x13x7")
			Expect(iopV13.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
limits:
  cpu: 7500m
  memory: "833"
requests:
  ephemeral-storage: 50Mi
  cpu: "3"
  memory: "333"
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x13x7")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x13x7
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

	Context("ingress gateway controller with inlet NodePort is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
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
			Expect(ingressVpa.Field("spec.resourcePolicy").String()).To(MatchJSON(`{"containerPolicies":[{"containerName":"istio-proxy","maxAllowed":{"cpu":"50m","memory":"200Mi"},"minAllowed":{"cpu":"10m","memory":"50Mi"}}]}`))

			Expect(ingressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"ingress-gateway-controller","heritage":"deckhouse","instance":"nodeport-test","istio.deckhouse.io/ingress-gateway-class":"np","module":"istio"}`))

			Expect(ingressSvc.Field("spec.type").String()).To(Equal("NodePort"))
		})
	})
	Context("ingress gateway controller with inlet LoadBalancer is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
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
			Expect(ingressVpa.Exists()).To(BeTrue())
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressSvc.Exists()).To(BeTrue())

			Expect(ingressVpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("Off"))

			Expect(ingressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"ingress-gateway-controller","heritage":"deckhouse","instance":"loadbalancer-test","istio.deckhouse.io/ingress-gateway-class":"lb","module":"istio"}`))

			Expect(ingressSvc.Field("metadata.annotations").String()).To(MatchJSON(`{ "aaa": "bbb" }`))
			Expect(ingressSvc.Field("spec.type").String()).To(Equal("LoadBalancer"))
		})
	})
	Context("ingress gateway controller with inlet HostPort is enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
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
			Expect(ingressVpa.Exists()).To(BeTrue())
			Expect(ingressDs.Exists()).To(BeTrue())
			Expect(ingressSvc.Exists()).To(BeTrue())

			Expect(ingressVpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("Off"))

			Expect(ingressDs.Field("metadata.labels").String()).To(MatchJSON(`{"app":"ingress-gateway-controller","heritage":"deckhouse","instance":"hostport-test","istio.deckhouse.io/ingress-gateway-class":"hp","module":"istio"}`))
			istioProxyContainer := ingressDs.Field("spec.template.spec.containers").Array()
			Expect(len(istioProxyContainer)).To(Equal(1))
			Expect((istioProxyContainer[0].Get("ports"))).To(MatchJSON(`[
{"containerPort":8080,"hostPort":80,"name":"http","protocol":"TCP"},
{"containerPort":8443,"hostPort":443,"name":"https","protocol":"TCP"},
{"containerPort":15090,"name":"http-envoy-prom","protocol":"TCP"},
{"containerPort":15021,"name":"status-port","protocol":"TCP"},
{"containerPort":15012,"name":"tls-istiod","protocol":"TCP"}
]`))

			Expect(ingressSvc.Field("spec.type").String()).To(Equal("ClusterIP"))
		})
	})

	Context("applicationNamespacesToMonitor are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.applicationNamespacesToMonitor", `
- "myns"
- "review-123"
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			podMonitor := f.KubernetesResource("podmonitor", "d8-monitoring", "istio-sidecars")

			Expect(podMonitor.Exists()).To(BeTrue())
			Expect(podMonitor.Field("spec.namespaceSelector.matchNames")).To(MatchJSON(`["myns","review-123"]`))
		})
	})
})
