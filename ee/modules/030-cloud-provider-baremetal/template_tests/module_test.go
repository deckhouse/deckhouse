/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
  enabledModules: ["cloud-provider-baremetal"]
  discovery:
    kubernetesVersion: 1.34.9
    clusterUUID: cluster
    d8SpecificNodeCountByRole:
      master: 1
`

var _ = Describe("Module :: cloud-provider-baremetal :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("cloudProviderBaremetal", `internal:
  baremetalOperatorWebhookCert:
    ca: bmo-ca
    crt: bmo-crt
    key: bmo-key
  ironicStandaloneOperatorWebhookCert:
    ca: irso-ca
    crt: irso-crt
    key: irso-key
  capm3WebhookCert:
    ca: capm3-ca
    crt: capm3-crt
    key: capm3-key
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
		Expect(providerRegistrationSecret.Field("data.instanceClassKind").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("BareMetalInstanceClass"))))
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

		clusterAdminRole := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:cloud-provider-baremetal:cluster-admin")
		Expect(clusterAdminRole.Field("rules").String()).To(ContainSubstring("baremetalimages"))
		Expect(clusterAdminRole.Field("rules").String()).To(ContainSubstring("baremetalramdiskimages"))

		ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-baremetal", "ironic")
		Expect(ironic.Exists()).To(BeFalse())
	})

	Context("with managed Ironic enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderBaremetal.nodes.parameters.ironic", `
provisioningNetwork:
  interface: eno3
  ipAddress: 172.22.0.20
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

			defaultRamdisk := f.KubernetesGlobalResource("BareMetalRamdiskImage", "baremetal-default-ramdisk")
			Expect(defaultRamdisk.Exists()).To(BeTrue())
			Expect(defaultRamdisk.Field("spec.direct.architecture").String()).To(Equal("x86_64"))
			Expect(defaultRamdisk.Field("spec.direct.kernelURL").String()).To(Equal("http://172.22.0.20:6180/images/ironic-python-agent.kernel"))
			Expect(defaultRamdisk.Field("spec.direct.initramfsURL").String()).To(Equal("http://172.22.0.20:6180/images/ironic-python-agent.initramfs"))

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-baremetal", "ironic")
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

			capm3 := f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "capm3-controller-manager")
			Expect(capm3.Exists()).To(BeTrue())
			capm3Args := capm3.Field("spec.template.spec.containers.0.args").String()
			Expect(capm3Args).To(ContainSubstring("--enableBMHNameBasedPreallocation=false"))
			Expect(capm3Args).To(ContainSubstring("--diagnostics-address=:8443"))
			Expect(capm3Args).To(ContainSubstring("--insecure-diagnostics=false"))
			Expect(capm3Args).To(ContainSubstring("--tls-min-version=VersionTLS13"))
			Expect(capm3Args).NotTo(ContainSubstring("${"))

			capm3FastTrackConfigMap := f.KubernetesResource("ConfigMap", "d8-cloud-provider-baremetal", "capm3-capm3fasttrack-configmap")
			Expect(capm3FastTrackConfigMap.Exists()).To(BeTrue())
			Expect(capm3FastTrackConfigMap.Field("data.CAPM3_FAST_TRACK").String()).To(Equal("false"))

			capm3WebhookService := f.KubernetesResource("Service", "d8-cloud-provider-baremetal", "capm3-webhook-service")
			Expect(capm3WebhookService.Exists()).To(BeTrue())
			Expect(capm3WebhookService.Field("spec.selector.control-plane").String()).To(Equal("controller-manager"))
			Expect(capm3WebhookService.Field("spec.selector.controller-tools\\.k8s\\.io").String()).To(Equal("1.0"))

			bmo := f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "baremetal-operator-controller-manager")
			Expect(bmo.Exists()).To(BeTrue())

			irso := f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "ironic-standalone-operator-controller-manager")
			Expect(irso.Exists()).To(BeTrue())
			irsoWebhookService := f.KubernetesResource("Service", "d8-cloud-provider-baremetal", "ironic-standalone-operator-webhook-service")
			Expect(irsoWebhookService.Exists()).To(BeTrue())
			Expect(irsoWebhookService.Field("spec.selector.app\\.kubernetes\\.io/part-of").String()).To(Equal("ironic-standalone-operator"))
			Expect(irso.Field("spec.template.metadata.labels.app\\.kubernetes\\.io/part-of").String()).To(Equal("ironic-standalone-operator"))

			instanceManager := f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "baremetal-instance-manager")
			Expect(instanceManager.Exists()).To(BeTrue())
			instanceManagerArgs := instanceManager.Field("spec.template.spec.containers.0.args").String()
			Expect(instanceManagerArgs).To(ContainSubstring("--target-namespace=d8-cloud-instance-manager"))
			Expect(instanceManagerArgs).To(ContainSubstring("--provider-namespace=d8-cloud-provider-baremetal"))

			instanceManagerRole := f.KubernetesResource("Role", "d8-cloud-instance-manager", "baremetal-instance-manager")
			Expect(instanceManagerRole.Exists()).To(BeTrue())
			instanceManagerProviderRole := f.KubernetesResource("Role", "d8-cloud-provider-baremetal", "baremetal-instance-manager")
			Expect(instanceManagerProviderRole.Exists()).To(BeTrue())
			instanceManagerRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "baremetal-instance-manager")
			Expect(instanceManagerRoleBinding.Exists()).To(BeTrue())
			instanceManagerProviderRoleBinding := f.KubernetesResource("RoleBinding", "d8-cloud-provider-baremetal", "baremetal-instance-manager-provider")
			Expect(instanceManagerProviderRoleBinding.Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "baremetal-instance-manager").Exists()).To(BeFalse())
			Expect(instanceManagerArgs).To(ContainSubstring("--bmc-probe-timeout=15s"))
			Expect(instanceManagerArgs).NotTo(ContainSubstring("--default-online"))
			Expect(instanceManagerArgs).NotTo(ContainSubstring("--default-disable-automated-cleaning"))

			bmoWebhook := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "baremetal-operator-validating-webhook-configuration")
			Expect(bmoWebhook.Exists()).To(BeTrue())
			Expect(bmoWebhook.Field("webhooks.0.clientConfig.service.namespace").String()).To(Equal("d8-cloud-provider-baremetal"))
			Expect(bmoWebhook.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("bmo-ca"))))

			bmoWebhookSecret := f.KubernetesResource("Secret", "d8-cloud-provider-baremetal", "bmo-webhook-server-cert")
			Expect(bmoWebhookSecret.Exists()).To(BeTrue())
			Expect(bmoWebhookSecret.Field("data.ca\\.crt").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("bmo-ca"))))

			irsoWebhook := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "ironic-standalone-operator-validating-webhook-configuration")
			Expect(irsoWebhook.Exists()).To(BeTrue())
			Expect(irsoWebhook.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("irso-ca"))))

			capm3Webhook := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "capm3-validating-webhook-configuration")
			Expect(capm3Webhook.Exists()).To(BeTrue())
			Expect(capm3Webhook.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("capm3-ca"))))

			capm3MutatingWebhook := f.KubernetesGlobalResource("MutatingWebhookConfiguration", "capm3-mutating-webhook-configuration")
			Expect(capm3MutatingWebhook.Exists()).To(BeTrue())
			Expect(capm3MutatingWebhook.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("capm3-ca"))))
		})

		It("renders custom BMC probe timeout for BareMetalInstance manager", func() {
			f.ValuesSet("cloudProviderBaremetal.nodes.parameters.ironic.bmcProbeTimeoutSeconds", 30)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			instanceManager := f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "baremetal-instance-manager")
			Expect(instanceManager.Exists()).To(BeTrue())
			instanceManagerArgs := instanceManager.Field("spec.template.spec.containers.0.args").String()
			Expect(instanceManagerArgs).To(ContainSubstring("--bmc-probe-timeout=30s"))
		})

		It("downloads custom UEFI iPXE firmware using the filename expected by httpd", func() {
			f.ValuesSet("cloudProviderBaremetal.nodes.parameters.ironic.provisioningTLS.certificateRef.name", "test-tls")
			f.ValuesSet("cloudProviderBaremetal.internal.resolvedIPXEFirmware", map[string]interface{}{
				"bios": map[string]interface{}{
					"url":    "https://images.example.test/undionly.kpxe",
					"sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				"uefiX86_64": map[string]interface{}{
					"url":    "https://images.example.test/snponly.efi",
					"sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				},
			})
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-baremetal", "ironic")
			downloaderArgs := ironic.Field("spec.overrides.initContainers.0.args.0").String()
			Expect(downloaderArgs).To(ContainSubstring("/shared/custom_ipxe_firmware/snponly.efi"))
			Expect(downloaderArgs).NotTo(ContainSubstring("snponly-x86_64.efi"))
		})
	})

	Context("with external DHCP configured", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderBaremetal.nodes.parameters.ironic", `
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

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-baremetal", "ironic")
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
			f.ValuesSetFromYaml("cloudProviderBaremetal.nodes.parameters.ironic", `
provisioningNetwork:
  interface: eno3
  ipAddress: 172.22.0.20
dhcp:
  internal:
    networkCIDR: 172.22.0.0/24
    rangeBegin: 172.22.0.200
    rangeEnd: 172.22.0.210
`)
			f.ValuesSetFromYaml("cloudProviderBaremetal.internal.resolvedRamdiskImage", `
direct:
  architecture: aarch64
  kernelURL: http://172.22.0.30/ipa/ironic-python-agent.kernel
  initramfsURL: http://172.22.0.30/ipa/ironic-python-agent.initramfs
`)
			f.HelmRender()
		})

		It("renders Ironic with custom IPA agent images", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-baremetal", "ironic")
			Expect(ironic.Exists()).To(BeTrue())
			Expect(ironic.Field("spec.deployRamdisk.disableDownloader").Bool()).To(BeTrue())
			Expect(ironic.Field("spec.overrides.agentImages.0.architecture").String()).To(Equal("aarch64"))
			Expect(ironic.Field("spec.overrides.agentImages.0.kernel").String()).To(Equal("http://172.22.0.30/ipa/ironic-python-agent.kernel"))
			Expect(ironic.Field("spec.overrides.agentImages.0.initramfs").String()).To(Equal("http://172.22.0.30/ipa/ironic-python-agent.initramfs"))
		})
	})

	Context("with an external Ironic instance", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderBaremetal.nodes.parameters.ironic", `
externalInstance:
  endpoint: https://external-ironic.example.com:6385/v1/
  credentialsRef:
    kind: Secret
    name: ironic-api-credentials
    namespace: d8-cloud-provider-baremetal
  tls:
    caCertRef:
      kind: Secret
      name: external-ironic-ca
      namespace: d8-cloud-provider-baremetal
      key: ca.crt
    clientCertRef:
      kind: Secret
      name: external-ironic-client
      namespace: d8-cloud-provider-baremetal
      certKey: tls.crt
      privateKey: tls.key
    insecure: false
`)
			f.HelmRender()
		})

		It("renders BMO and CAPM3 without the managed Ironic stack", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Ironic", "d8-cloud-provider-baremetal", "ironic").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "ironic-standalone-operator-controller-manager").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "baremetal-operator-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "capm3-controller-manager").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "baremetal-instance-manager").Exists()).To(BeTrue())

			config := f.KubernetesResource("ConfigMap", "d8-cloud-provider-baremetal", "ironic")
			Expect(config.Field("data.IRONIC_ENDPOINT").String()).To(Equal("https://external-ironic.example.com:6385/v1/"))
			Expect(config.Field("data.IRONIC_CACERT_FILE").String()).To(Equal("/opt/metal3/external-ironic/ca/ca.crt"))
			Expect(config.Field("data.IRONIC_CLIENT_CERT_FILE").String()).To(Equal("/opt/metal3/external-ironic/client/tls.crt"))
			Expect(config.Field("data.IRONIC_CLIENT_PRIVATE_KEY_FILE").String()).To(Equal("/opt/metal3/external-ironic/client/tls.key"))

			bmo := f.KubernetesResource("Deployment", "d8-cloud-provider-baremetal", "baremetal-operator-controller-manager")
			Expect(bmo.Field("spec.template.spec.volumes.1.secret.secretName").String()).To(Equal("ironic-api-credentials"))
			Expect(bmo.Field("spec.template.spec.volumes.2.secret.secretName").String()).To(Equal("external-ironic-ca"))
			Expect(bmo.Field("spec.template.spec.volumes.3.secret.secretName").String()).To(Equal("external-ironic-client"))
		})
	})
})
