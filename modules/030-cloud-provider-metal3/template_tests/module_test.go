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
`

var _ = Describe("Module :: cloud-provider-metal3 :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
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

		ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic")
		Expect(ironic.Exists()).To(BeFalse())
	})

	Context("with managed Ironic enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("cloudProviderMetal3.ironic", `
enabled: true
version: "34.0"
deployRamdisk:
  sshKey: ssh-ed25519 AAAAC3Nz
networking:
  interface: eno3
  ipAddress: 172.22.0.20
  ipAddressManager: keepalived
  dhcp:
    networkCIDR: 172.22.0.0/24
    rangeBegin: 172.22.0.200
    rangeEnd: 172.22.0.210
    dnsAddress: 10.222.0.10
    gatewayAddress: 172.22.0.20
`)
			f.HelmRender()
		})

		It("renders Ironic with DHCP DNS and gateway settings", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ironic := f.KubernetesResource("Ironic", "d8-cloud-provider-metal3", "ironic")
			Expect(ironic.Exists()).To(BeTrue())
			Expect(ironic.Field("spec.version").String()).To(Equal("34.0"))
			Expect(ironic.Field("spec.deployRamdisk.sshKey").String()).To(Equal("ssh-ed25519 AAAAC3Nz"))
			Expect(ironic.Field("spec.networking.interface").String()).To(Equal("eno3"))
			Expect(ironic.Field("spec.networking.ipAddress").String()).To(Equal("172.22.0.20"))
			Expect(ironic.Field("spec.networking.ipAddressManager").String()).To(Equal("keepalived"))
			Expect(ironic.Field("spec.networking.dhcp.networkCIDR").String()).To(Equal("172.22.0.0/24"))
			Expect(ironic.Field("spec.networking.dhcp.rangeBegin").String()).To(Equal("172.22.0.200"))
			Expect(ironic.Field("spec.networking.dhcp.rangeEnd").String()).To(Equal("172.22.0.210"))
			Expect(ironic.Field("spec.networking.dhcp.dnsAddress").String()).To(Equal("10.222.0.10"))
			Expect(ironic.Field("spec.networking.dhcp.gatewayAddress").String()).To(Equal("172.22.0.20"))
		})
	})
})
