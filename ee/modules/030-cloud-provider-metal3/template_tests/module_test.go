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
	"encoding/base64"
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

const providerID = "metal3"
const nameLabelKey = "cloud-provider\\.deckhouse\\.io/name"
const registrationLabelKey = "cloud-provider\\.deckhouse\\.io/registration"
const ephemeralNodesTemplatesLabelKey = "cloud-provider\\.deckhouse\\.io/ephemeral-nodes-templates"

const globalValues = `
  clusterIsBootstrapped: true
  enabledModules: ["cloud-provider-metal3"]
  discovery:
    kubernetesVersion: 1.34.9
    clusterUUID: cluster
    d8SpecificNodeCountByRole:
      master: 1
`

var _ = Describe("Module :: cloud-provider-metal3 :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("cloudProviderMetal3", `internal:
  providerDiscoveryData:
    zones:
    - provisioning
`)
		f.HelmRender()
	})

	It("renders registration and CAPI template secrets", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		providerRegistrationSecret := f.KubernetesResource("Secret", "kube-system", "d8-node-manager-cloud-provider")
		Expect(providerRegistrationSecret.Exists()).To(BeTrue())
		Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", registrationLabelKey)).String()).To(Equal(""))
		Expect(providerRegistrationSecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
		Expect(providerRegistrationSecret.Field("data.type").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))
		Expect(providerRegistrationSecret.Field("data.instanceClassKind").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("Metal3InstanceClass"))))
		Expect(providerRegistrationSecret.Field("data.capiClusterKind").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("Metal3Cluster"))))
		Expect(providerRegistrationSecret.Field("data.capiMachineTemplateKind").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("Metal3MachineTemplate"))))

		providerSpecificRegistrationSecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-node-manager-cloud-provider-%s", providerID))
		Expect(providerSpecificRegistrationSecret.Exists()).To(BeTrue())
		Expect(providerSpecificRegistrationSecret.Field("data.capiClusterName").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(providerID))))

		providerSpecificCAPISecret := f.KubernetesResource("Secret", "kube-system", fmt.Sprintf("d8-cloud-provider-%s-capi", providerID))
		Expect(providerSpecificCAPISecret.Exists()).To(BeTrue())
		Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", ephemeralNodesTemplatesLabelKey)).String()).To(Equal("capi"))
		Expect(providerSpecificCAPISecret.Field(fmt.Sprintf("metadata.labels.%s", nameLabelKey)).String()).To(Equal(providerID))
		Expect(providerSpecificCAPISecret.Field("data.cluster\\.yaml").String()).NotTo(BeEmpty())
		Expect(providerSpecificCAPISecret.Field("data.machine-template\\.yaml").String()).NotTo(BeEmpty())
		Expect(providerSpecificCAPISecret.Field("data.instance-class\\.checksum").String()).NotTo(BeEmpty())

		clusterAdminRole := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-metal3:cluster-admin")
		Expect(clusterAdminRole.Field("rules").String()).To(ContainSubstring("metal3images"))
		Expect(clusterAdminRole.Field("rules").String()).To(ContainSubstring("metal3ramdiskimages"))

		ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic")
		Expect(ironic.Exists()).To(BeFalse())
	})

	Context("with managed Ironic enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderMetal3.nodes.parameters.ironic", `
provisioningNetwork:
  interface: eno3
  ipAddress: 172.22.0.20
  ipAddressManager: keepalived
dhcp:
  internal:
    networkCIDR: 172.22.0.0/24
    rangeBegin: 172.22.0.200
    rangeEnd: 172.22.0.210
    dnsAddress: 10.222.0.10
    gatewayAddress: 172.22.0.20
    serveDNS: false
`)
			f.HelmRender()
		})

		It("renders Ironic with DHCP DNS and gateway settings", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic")
			Expect(ironic.Exists()).To(BeTrue())
			Expect(ironic.Field("spec.images.ironic").String()).NotTo(BeEmpty())
			Expect(ironic.Field("spec.deployRamdisk.disableDownloader").Bool()).To(BeTrue())
			Expect(ironic.Field("spec.overrides.agentImages.0.architecture").String()).To(Equal("x86_64"))
			Expect(ironic.Field("spec.overrides.agentImages.0.kernel").String()).To(Equal("http://172.22.0.20:6180/images/ironic-python-agent.kernel"))
			Expect(ironic.Field("spec.overrides.agentImages.0.initramfs").String()).To(Equal("http://172.22.0.20:6180/images/ironic-python-agent.initramfs"))
			Expect(ironic.Field("spec.networking.interface").String()).To(Equal("eno3"))
			Expect(ironic.Field("spec.networking.ipAddress").String()).To(Equal("172.22.0.20"))
			Expect(ironic.Field("spec.networking.ipAddressManager").String()).To(Equal("keepalived"))
			Expect(ironic.Field("spec.networking.dhcp.networkCIDR").String()).To(Equal("172.22.0.0/24"))
			Expect(ironic.Field("spec.networking.dhcp.rangeBegin").String()).To(Equal("172.22.0.200"))
			Expect(ironic.Field("spec.networking.dhcp.rangeEnd").String()).To(Equal("172.22.0.210"))
			Expect(ironic.Field("spec.networking.dhcp.dnsAddress").String()).To(Equal("10.222.0.10"))
			Expect(ironic.Field("spec.networking.dhcp.gatewayAddress").String()).To(Equal("172.22.0.20"))

			capm3 := f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "capm3-controller-manager")
			Expect(capm3.Exists()).To(BeTrue())
			capm3Args := capm3.Field("spec.template.spec.containers.0.args").String()
			Expect(capm3Args).To(ContainSubstring("--enableBMHNameBasedPreallocation=false"))
			Expect(capm3Args).To(ContainSubstring("--diagnostics-address=:8443"))
			Expect(capm3Args).To(ContainSubstring("--insecure-diagnostics=false"))
			Expect(capm3Args).To(ContainSubstring("--tls-min-version=VersionTLS13"))
			Expect(capm3Args).NotTo(ContainSubstring("${"))

			capm3FastTrackConfigMap := f.KubernetesResource("ConfigMap", "d8-cloud-provider-metal3", "capm3-capm3fasttrack-configmap")
			Expect(capm3FastTrackConfigMap.Exists()).To(BeTrue())
			Expect(capm3FastTrackConfigMap.Field("data.CAPM3_FAST_TRACK").String()).To(Equal("false"))

			capm3WebhookService := f.KubernetesResource("Service", "d8-cloud-provider-metal3", "capm3-webhook-service")
			Expect(capm3WebhookService.Exists()).To(BeTrue())
			Expect(capm3WebhookService.Field("spec.selector.control-plane").String()).To(Equal("controller-manager"))
			Expect(capm3WebhookService.Field("spec.selector.controller-tools\\.k8s\\.io").String()).To(Equal("1.0"))

			bmo := f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "baremetal-operator-controller-manager")
			Expect(bmo.Exists()).To(BeTrue())

			irso := f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "ironic-standalone-operator-controller-manager")
			Expect(irso.Exists()).To(BeTrue())

			instanceManager := f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "metal3-instance-manager")
			Expect(instanceManager.Exists()).To(BeTrue())
			instanceManagerArgs := instanceManager.Field("spec.template.spec.containers.0.args").String()
			Expect(instanceManagerArgs).To(ContainSubstring("--target-namespace=d8-cloud-instance-manager"))
			Expect(instanceManagerArgs).NotTo(ContainSubstring("--default-online"))
			Expect(instanceManagerArgs).NotTo(ContainSubstring("--default-disable-automated-cleaning"))

			bmoWebhook := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "baremetal-operator-validating-webhook-configuration")
			Expect(bmoWebhook.Exists()).To(BeTrue())
			Expect(bmoWebhook.Field("webhooks.0.clientConfig.service.namespace").String()).To(Equal("d8-cloud-provider-metal3"))
		})
	})

	Context("with external DHCP configured", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderMetal3.nodes.parameters.ironic", `
provisioningNetwork:
  interface: eno3
  ipAddress: 172.22.0.20
dhcp:
  external:
    pxeBootServer: 172.22.0.20
    pxeBootFile:
      bios: undionly.kpxe
      uefi: snponly.efi
`)
			f.HelmRender()
		})

		It("renders Ironic without managed DHCP settings", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic")
			Expect(ironic.Exists()).To(BeTrue())
			Expect(ironic.Field("spec.networking.interface").String()).To(Equal("eno3"))
			Expect(ironic.Field("spec.networking.ipAddress").String()).To(Equal("172.22.0.20"))
			Expect(ironic.Field("spec.deployRamdisk.disableDownloader").Bool()).To(BeTrue())
			Expect(ironic.Field("spec.overrides.agentImages.0.kernel").String()).To(Equal("http://172.22.0.20:6180/images/ironic-python-agent.kernel"))
			Expect(ironic.Field("spec.overrides.agentImages.0.initramfs").String()).To(Equal("http://172.22.0.20:6180/images/ironic-python-agent.initramfs"))
			Expect(ironic.Field("spec.networking.dhcp").Exists()).To(BeFalse())
		})
	})

	Context("with a resolved custom ramdisk image", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderMetal3.nodes.parameters.ironic", `
provisioningNetwork:
  interface: eno3
  ipAddress: 172.22.0.20
dhcp:
  internal:
    networkCIDR: 172.22.0.0/24
    rangeBegin: 172.22.0.200
    rangeEnd: 172.22.0.210
`)
			f.ValuesSetFromYaml("cloudProviderMetal3.internal.ramdiskImage", `
direct:
  architecture: aarch64
  kernelURL: http://172.22.0.30/ipa/ironic-python-agent.kernel
  initramfsURL: http://172.22.0.30/ipa/ironic-python-agent.initramfs
`)
			f.HelmRender()
		})

		It("renders Ironic with custom IPA agent images", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic")
			Expect(ironic.Exists()).To(BeTrue())
			Expect(ironic.Field("spec.deployRamdisk.disableDownloader").Bool()).To(BeTrue())
			Expect(ironic.Field("spec.overrides.agentImages.0.architecture").String()).To(Equal("aarch64"))
			Expect(ironic.Field("spec.overrides.agentImages.0.kernel").String()).To(Equal("http://172.22.0.30/ipa/ironic-python-agent.kernel"))
			Expect(ironic.Field("spec.overrides.agentImages.0.initramfs").String()).To(Equal("http://172.22.0.30/ipa/ironic-python-agent.initramfs"))
		})
	})

	Context("with an external Ironic instance", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderMetal3.nodes.parameters.ironic", `
externalInstance:
  endpoint: https://external-ironic.example.com:6385/v1/
  credentialsRef:
    kind: Secret
    name: ironic-api-credentials
    namespace: d8-cloud-provider-metal3
  tls:
    caCertRef:
      kind: Secret
      name: external-ironic-ca
      namespace: d8-cloud-provider-metal3
      key: ca.crt
    clientCertRef:
      kind: Secret
      name: external-ironic-client
      namespace: d8-cloud-provider-metal3
      certKey: tls.crt
      privateKey: tls.key
    insecure: false
`)
			f.HelmRender()
		})

		It("renders BMO and CAPM3 without the managed Ironic stack", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "ironic-standalone-operator-controller-manager").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "baremetal-operator-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "capm3-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "metal3-instance-manager").Exists()).To(BeTrue())

			config := f.KubernetesResource("ConfigMap", "d8-cloud-provider-metal3", "ironic")
			Expect(config.Field("data.IRONIC_ENDPOINT").String()).To(Equal("https://external-ironic.example.com:6385/v1/"))
			Expect(config.Field("data.IRONIC_CACERT_FILE").String()).To(Equal("/opt/metal3/external-ironic/ca/ca.crt"))
			Expect(config.Field("data.IRONIC_CLIENT_CERT_FILE").String()).To(Equal("/opt/metal3/external-ironic/client/tls.crt"))
			Expect(config.Field("data.IRONIC_CLIENT_PRIVATE_KEY_FILE").String()).To(Equal("/opt/metal3/external-ironic/client/tls.key"))

			bmo := f.KubernetesResource("Deployment", "d8-cloud-provider-metal3", "baremetal-operator-controller-manager")
			Expect(bmo.Field("spec.template.spec.volumes.1.secret.secretName").String()).To(Equal("ironic-api-credentials"))
			Expect(bmo.Field("spec.template.spec.volumes.2.secret.secretName").String()).To(Equal("external-ironic-ca"))
			Expect(bmo.Field("spec.template.spec.volumes.3.secret.secretName").String()).To(Equal("external-ironic-client"))
		})
	})
})
