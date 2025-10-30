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
enabledModules: ["operator-prometheus","cert-manager","vertical-pod-autoscaler","cni-cilium"]
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
      globalVersion: "1.21.6"
      versionMap:
        "1.21.6":
          revision: "v1x21x6"
          fullVersion: "1.21.6"
          imageSuffix: "V1x21x6"
        "1.19.7":
          revision: "v1x19x7"
          fullVersion: "1.19.7"
          imageSuffix: "V1x19x7"
      kialiSigningKey: "kiali"
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
      proxyConfig: {}
      accessLog:
        type: "Text"
        textFormat: '[%START_TIME%] "%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%" %RESPONSE_CODE% %RESPONSE_FLAGS% %RESPONSE_CODE_DETAILS% %CONNECTION_TERMINATION_DETAILS% "%UPSTREAM_TRANSPORT_FAILURE_REASON%" %BYTES_RECEIVED% %BYTES_SENT% %DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)% "%REQ(X-FORWARDED-FOR)%" "%REQ(USER-AGENT)%" "%REQ(X-REQUEST-ID)%" "%REQ(:AUTHORITY)%" "%UPSTREAM_HOST%" %UPSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_REMOTE_ADDRESS% %REQUESTED_SERVER_NAME% %ROUTE_NAME%'
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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6","1.19.7"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.21.6","1.19.7"]`)
			f.ValuesSetFromYaml("istio.internal.applicationNamespaces", `[foo,bar]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			mwh := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-istio-sidecar-injector-global")
			Expect(mwh.Exists()).To(BeTrue())
			Expect(len(mwh.Field("webhooks").Array())).To(Equal(2))

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			iopV19 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x19x7")

			deploymentOperatorV21 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x21x6")
			deploymentOperatorV19 := f.KubernetesResource("Deployment", "d8-istio", "operator-v1x19x7")

			secretD8RegistryFoo := f.KubernetesResource("Secret", "foo", "d8-istio-sidecar-registry")
			secretD8RegistryBar := f.KubernetesResource("Secret", "bar", "d8-istio-sidecar-registry")

			secretCacerts := f.KubernetesResource("Secret", "d8-istio", "cacerts")

			serviceGlobal := f.KubernetesResource("Service", "d8-istio", "istiod")

			Expect(iopV21.Exists()).To(BeTrue())
			Expect(iopV19.Exists()).To(BeTrue())
			Expect(deploymentOperatorV21.Exists()).To(BeTrue())
			Expect(deploymentOperatorV19.Exists()).To(BeTrue())
			Expect(secretCacerts.Exists()).To(BeTrue())

			Expect(secretD8RegistryFoo.Exists()).To(BeTrue())
			Expect(secretD8RegistryBar.Exists()).To(BeTrue())

			Expect(mwh.Exists()).To(BeTrue())
			Expect(serviceGlobal.Exists()).To(BeTrue())

			Expect(iopV21.Field("spec.revision").String()).To(Equal(`v1x21x6`))
			Expect(iopV19.Field("spec.revision").String()).To(Equal(`v1x19x7`))

			Expect(iopV21.Field("spec.meshConfig.rootNamespace").String()).To(Equal(`d8-istio`))
			Expect(iopV19.Field("spec.meshConfig.rootNamespace").String()).To(Equal(`d8-istio`))

			Expect(deploymentOperatorV21.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.example.com@imageHash-istio-operatorV1x21x6`))
			Expect(deploymentOperatorV19.Field("spec.template.spec.containers.0.image").String()).To(Equal(`registry.example.com@imageHash-istio-operatorV1x19x7`))

			Expect(iopV21.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.example.com@imageHash-istio-proxyv2V1x21x6`))
			Expect(iopV19.Field("spec.values.global.proxy.image").String()).To(Equal(`registry.example.com@imageHash-istio-proxyv2V1x19x7`))

			Expect(iopV21.Field("spec.values.pilot.image").String()).To(Equal(`registry.example.com@imageHash-istio-pilotV1x21x6`))
			Expect(iopV19.Field("spec.values.pilot.image").String()).To(Equal(`registry.example.com@imageHash-istio-pilotV1x19x7`))

			Expect(mwh.Field("webhooks.0.clientConfig.service.name").String()).To(Equal(`istiod-v1x21x6`))
			Expect(mwh.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(`bXljZXJ0`)) // b64("mycert")
			Expect(serviceGlobal.Field("spec.selector").String()).To(MatchJSON(`{"app":"istiod","istio.io/rev":"v1x21x6"}`))

			Expect(secretCacerts.Field("data").String()).To(MatchJSON(`
				{
					"ca-cert.pem":"bXljZXJ0",
					"ca-key.pem":"bXlrZXk=",
					"cert-chain.pem":"bXljaGFpbg==",
					"root-cert.pem":"bXlyb290"
				}
`))

			Expect(iopV21.Field("spec.meshConfig.caCertificates").Exists()).To(BeFalse())
			Expect(iopV21.Field("spec.values.meshNetworks").Exists()).To(BeFalse())

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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.21.6"]`)
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
              protocol:
            `))
			Expect(se.Field("spec.endpoints").String()).To(MatchYAML(`
            - address: 1.1.1.1
              ports:
                aaa: 123
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

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.meshConfig.caCertificates").String()).To(MatchJSON(`[{"pem": "---ROOT CA---"}]`))
			Expect(iopV21.Field("spec.values.meshNetworks").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "istio-ingressgateway").Exists()).To(BeTrue())
		})
	})

	Context("Cloud provider OpenStack", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.21.6"]`)
			f.HelmRender()
		})
		It("CLOUD_PROVIDER env should be 'none'", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Exists()).To(BeTrue())
			Expect(iopV21.Field("spec.meshConfig.defaultConfig.proxyMetadata.CLOUD_PLATFORM").String()).To(Equal("none"))
		})
	})

	Context("Cloud provider AWS", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.clusterConfiguration.cloud.provider", "AWS")
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.21.6"]`)
			f.HelmRender()
		})
		It("CLOUD_PROVIDER env should be 'aws'", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Exists()).To(BeTrue())
			Expect(iopV21.Field("spec.meshConfig.defaultConfig.proxyMetadata.CLOUD_PLATFORM").String()).To(Equal("aws"))
		})
	})

	Context("There are some multiclusters, multiclustersNeedIngressGateway = true", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.ValuesSetFromYaml("istio.internal.operatorVersionsToInstall", `["1.21.6"]`)
			f.ValuesSet("istio.multicluster.enabled", true)
			f.ValuesSet("istio.internal.multiclustersNeedIngressGateway", true)
			f.ValuesSetFromYaml("istio.internal.multiclusters", `
- name: neighbour-0
  spiffeEndpoint: https://some-proper-host/spiffe-bundle-endpoint
  enableIngressGateway: true
  apiHost: remote.api.example.com
  insecureSkipVerify: false
  ca: |
    -----BEGIN CERTIFICATE-----
    MIIFDTCCAvWgAwIBAgIURb+L4dH5qv53ZSD/tBRSdq37Go4wDQYJKoZIhvcNAQEL
    BQAwFjEUMBIGA1UEAwwLY3VzdG9tLXJvb3QwHhcNMjQxMTI1MTUyNzMwWhcNMzQx
    MTIzMTUyNzMwWjAWMRQwEgYDVQQDDAtjdXN0b20tcm9vdDCCAiIwDQYJKoZIhvcN
    AQEBBQADggIPADCCAgoCggIBAJ4BNx4E5e9EHzVIrz373AOanQsOMGBw/SxzfiPe
    9PlNtvG9YXMtYljsonbZI420b8E8yWCE+EzRj0Xut8ypn1uB7+PvVUgb2TChHnWv
    6CvEhiyCBquOGKa2nKvMlov5SGscj/CUyj+xDxvoTvPWo1UxWvjjhC7zTG5BGiBx
    ltBJb2oKsgg3zDG74X4htBNW/QMuxYps9mTNuwI6970eqEZ81x43+66hGWSnyd3Y
    3fz8S/EqKz3EvPFUN44oMiVRNVJq6q3r01tQjmGQ+hztpQZM4TZSZUAzqKfRqXuz
    GtxhYzQDPt2/aPWLU2yKDig9bneu4lMChDDnowk1XHkOOZK4Q5eQ2AcM0tOl37F2
    r34VmEGDbZ89o314EPfU8k9vNcGiSICxknW0LDdQXxF/k8FXmrTIgcEQsFTkVrPK
    H6UpSUvJZfU4LV6Li3/Xza+QUNGTlmq8RrE1tzEdFJfXbdHybhofIqjDctA0tlt7
    FpPjFe9CTB2SdqtmJO7ZssIJ1JITxTl9L7Z2BPyjFpX3zwRRA0mJH27jrW6Mx3Mm
    uHvOgrqp9D85IZI9QWsYuta/kKufSHhHYberWZPt3GrLAdQm1xCpTgu9cIQmUi0y
    oT4GPK54wmEXyUc0HidxJAGzIBlujUGrOn05mpw6VQpOfejTz5xqbvZAFhUG0Qqv
    gCeHAgMBAAGjUzBRMB0GA1UdDgQWBBS8/sU+MI1T80lnnRY7nVrZKJ1+WTAfBgNV
    HSMEGDAWgBS8/sU+MI1T80lnnRY7nVrZKJ1+WTAPBgNVHRMBAf8EBTADAQH/MA0G
    CSqGSIb3DQEBCwUAA4ICAQA8Ve8J1C9HMZw66INmYanyCSgYl5IeC7Qx7ot2ax4Y
    oxrBH1hV5pEDo8PQUMONQiEUu0HL+QO6yZEX1Dqbb2lIvGkZvZDOuZnQLuyWvSEt
    NuAbJrQMyFVpxftKt9aVL0I/NKDYkJvCWbI/PaeXd49HyU759HS7nurXu4uGSjGa
    YBdr0HYAseT2kEY5fPg9j09oWLYFEgjByWVGcLIwZZ1z14D68omfMa/kksBOtWnP
    SJIDmFZH+B85yY+w6yKb9/RfC7D7LYCETg+gIImuEyIqzLk5D4zGEvV/I99YwJSI
    mkzKIJ/4gkAVNqGTUZ6MBu7lCOcBLeQkcrViJEDL6ZMkmEyeT0zrvn+McSl5qw/x
    UNAW5/jQhyM/V4tAcGqJjBxI9cQbT4+FbDwwd9jQ5u4YQplEP5JLKPUG1hKCKdk6
    hNBAEwcIPz3iH+hEKZAvVnblfZKxpIMHeLtxmf8K/2JvJVTJnHmYxvK34A+XhErd
    8AyIVpdSYwV1NsSTgMHPdPePZA+H3mRQOtTt1ad8lEg2Lakxtun7kQO61K5yTXcN
    /QP7w+GMvdsqQm8EoqDXPz8Cfno+it942Reb2zmDGrGfh+/inoM/8ACvhNAz4Qyc
    ejEkdtt+SM9ao/txR3M/3t/IiAyY9lGT+N3VePEo6UNyfSRdSCT3c4Y/NUs4yXHS
    tQ==
    -----END CERTIFICATE-----
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
    insecure-skip-tls-verify: false
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUZEVENDQXZXZ0F3SUJBZ0lVUmIrTDRkSDVxdjUzWlNEL3RCUlNkcTM3R280d0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZqRVVNQklHQTFVRUF3d0xZM1Z6ZEc5dExYSnZiM1F3SGhjTk1qUXhNVEkxTVRVeU56TXdXaGNOTXpReApNVEl6TVRVeU56TXdXakFXTVJRd0VnWURWUVFEREF0amRYTjBiMjB0Y205dmREQ0NBaUl3RFFZSktvWklodmNOCkFRRUJCUUFEZ2dJUEFEQ0NBZ29DZ2dJQkFKNEJOeDRFNWU5RUh6VklyejM3M0FPYW5Rc09NR0J3L1N4emZpUGUKOVBsTnR2RzlZWE10WWxqc29uYlpJNDIwYjhFOHlXQ0UrRXpSajBYdXQ4eXBuMXVCNytQdlZVZ2IyVENoSG5Xdgo2Q3ZFaGl5Q0JxdU9HS2Eybkt2TWxvdjVTR3Njai9DVXlqK3hEeHZvVHZQV28xVXhXdmpqaEM3elRHNUJHaUJ4Cmx0QkpiMm9Lc2dnM3pERzc0WDRodEJOVy9RTXV4WXBzOW1UTnV3STY5NzBlcUVaODF4NDMrNjZoR1dTbnlkM1kKM2Z6OFMvRXFLejNFdlBGVU40NG9NaVZSTlZKcTZxM3IwMXRRam1HUStoenRwUVpNNFRaU1pVQXpxS2ZScVh1egpHdHhoWXpRRFB0Mi9hUFdMVTJ5S0RpZzlibmV1NGxNQ2hERG5vd2sxWEhrT09aSzRRNWVRMkFjTTB0T2wzN0YyCnIzNFZtRUdEYlo4OW8zMTRFUGZVOGs5dk5jR2lTSUN4a25XMExEZFFYeEYvazhGWG1yVElnY0VRc0ZUa1ZyUEsKSDZVcFNVdkpaZlU0TFY2TGkzL1h6YStRVU5HVGxtcThSckUxdHpFZEZKZlhiZEh5YmhvZklxakRjdEEwdGx0NwpGcFBqRmU5Q1RCMlNkcXRtSk83WnNzSUoxSklUeFRsOUw3WjJCUHlqRnBYM3p3UlJBMG1KSDI3anJXNk14M01tCnVIdk9ncnFwOUQ4NUlaSTlRV3NZdXRhL2tLdWZTSGhIWWJlcldaUHQzR3JMQWRRbTF4Q3BUZ3U5Y0lRbVVpMHkKb1Q0R1BLNTR3bUVYeVVjMEhpZHhKQUd6SUJsdWpVR3JPbjA1bXB3NlZRcE9mZWpUejV4cWJ2WkFGaFVHMFFxdgpnQ2VIQWdNQkFBR2pVekJSTUIwR0ExVWREZ1FXQkJTOC9zVStNSTFUODBsbm5SWTduVnJaS0oxK1dUQWZCZ05WCkhTTUVHREFXZ0JTOC9zVStNSTFUODBsbm5SWTduVnJaS0oxK1dUQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01BMEcKQ1NxR1NJYjNEUUVCQ3dVQUE0SUNBUUE4VmU4SjFDOUhNWnc2NklObVlhbnlDU2dZbDVJZUM3UXg3b3QyYXg0WQpveHJCSDFoVjVwRURvOFBRVU1PTlFpRVV1MEhMK1FPNnlaRVgxRHFiYjJsSXZHa1p2WkRPdVpuUUx1eVd2U0V0Ck51QWJKclFNeUZWcHhmdEt0OWFWTDBJL05LRFlrSnZDV2JJL1BhZVhkNDlIeVU3NTlIUzdudXJYdTR1R1NqR2EKWUJkcjBIWUFzZVQya0VZNWZQZzlqMDlvV0xZRkVnakJ5V1ZHY0xJd1paMXoxNEQ2OG9tZk1hL2trc0JPdFduUApTSklEbUZaSCtCODV5WSt3NnlLYjkvUmZDN0Q3TFlDRVRnK2dJSW11RXlJcXpMazVENHpHRXZWL0k5OVl3SlNJCm1rektJSi80Z2tBVk5xR1RVWjZNQnU3bENPY0JMZVFrY3JWaUpFREw2Wk1rbUV5ZVQwenJ2bitNY1NsNXF3L3gKVU5BVzUvalFoeU0vVjR0QWNHcUpqQnhJOWNRYlQ0K0ZiRHd3ZDlqUTV1NFlRcGxFUDVKTEtQVUcxaEtDS2RrNgpoTkJBRXdjSVB6M2lIK2hFS1pBdlZuYmxmWkt4cElNSGVMdHhtZjhLLzJKdkpWVEpuSG1ZeHZLMzRBK1hoRXJkCjhBeUlWcGRTWXdWMU5zU1RnTUhQZFBlUFpBK0gzbVJRT3RUdDFhZDhsRWcyTGFreHR1bjdrUU82MUs1eVRYY04KL1FQN3crR012ZHNxUW04RW9xRFhQejhDZm5vK2l0OTQyUmViMnptREdyR2ZoKy9pbm9NLzhBQ3ZoTkF6NFF5YwplakVrZHR0K1NNOWFvL3R4UjNNLzN0L0lpQXlZOWxHVCtOM1ZlUEVvNlVOeWZTUmRTQ1QzYzRZL05VczR5WEhTCnRRPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
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

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.meshConfig.caCertificates").String()).To(MatchJSON(`[{"pem": "---ROOT CA---"}]`))
			Expect(iopV21.Field("spec.values.global.meshNetworks").String()).To(MatchYAML(`
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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
requests:
  cpu: 50m
  memory: 256Mi
  ephemeral-storage: 50Mi
limits: {}
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x21x6")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x21x6
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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.ValuesSetFromYaml("istio.controlPlane.resourcesManagement", `
mode: Static
static:
  requests:
    cpu: 11m
    memory: 22Mi
  limits:
    cpu: "33"
    memory: 44Gi
`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
requests:
  cpu: 11m
  memory: 22Mi
  ephemeral-storage: 50Mi
limits:
  cpu: "33"
  memory: 44Gi
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x21x6")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x21x6
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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
			f.HelmRender()
		})

		It("", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.values.global.proxy.resources").String()).To(MatchYAML(`{}`))
		})
	})

	Context("istiod with sidecar custom static resourcesManagement configuration", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("istio", istioValues)
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
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
			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.values.global.proxy.resources").String()).To(MatchYAML(`
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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
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

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
limits:
  cpu: 253m
  memory: "1342177280"
requests:
  ephemeral-storage: 50Mi
  cpu: 101m
  memory: 512Mi
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x21x6")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x21x6
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
			f.ValuesSetFromYaml("istio.internal.versionsToInstall", `["1.21.6"]`)
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

			iopV21 := f.KubernetesResource("IstioOperator", "d8-istio", "v1x21x6")
			Expect(iopV21.Field("spec.values.pilot.resources").String()).To(MatchYAML(`
limits:
  cpu: 7500m
  memory: "833"
requests:
  ephemeral-storage: 50Mi
  cpu: "3"
  memory: "333"
`))
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "istiod-v1x21x6")
			Expect(vpa.Field("spec").String()).To(MatchYAML(`
targetRef:
  apiVersion: apps/v1
  kind: Deployment
  name: istiod-v1x21x6
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
