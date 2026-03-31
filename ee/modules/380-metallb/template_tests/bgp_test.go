/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	bgpPeers = `
- name: bgp-peer-0
  peerAddress: 1.1.1.1
  peerASN: 65000
  myASN: 64000
  routerID: 10.0.0.254
  holdTime: 3s
- name: bgp-peer-1
  peerAddress: 1.1.1.2
  peerASN: 65000
  myASN: 64000
  holdTime: 3s
  nodeSelectors:
  - matchLabels:
      node: "metallb"
`

	addressPools = `
- name: mypool1
  addresses:
  - 192.168.0.0/24
- name: mypool2
  addresses:
  - 192.68.1.1-192.168.1.255
`

	bgpAdvertisements = `
- name: mypool1-0
  ipAddressPools:
  - mypool1
- name: mypool2-0
  ipAddressPools:
  - mypool2
  aggregationLength: 32
  localPref: 100
  communities:
  - "65535:65282"
  - "1111:1111"
- name: mypool2-1
  ipAddressPools:
  - mypool2
  aggregationLength: 32
  localPref: 150
`

	chartFile = "/deckhouse/ee/modules/380-metallb/Chart.yaml"
)

var _ = Describe("Module :: metallb :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeSuite(func() {
		err := os.WriteFile(chartFile, []byte("name: metallb\nversion: 0.0.1"), 0666)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove(chartFile)
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		f.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		f.ValuesSet("global.modulesImages.registry.base", "registryAddr")

		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
		f.ValuesSet("metallb.speaker.nodeSelector.mylabel", "myvalue")
	})

	Context("bgpPeers and addressPools in BGP mode are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("metallb.internal.bgpPeers", bgpPeers)
			f.ValuesSetFromYaml("metallb.internal.addressPools", addressPools)
			f.ValuesSetFromYaml("metallb.internal.bgpAdvertisements", bgpAdvertisements)
			f.ValuesSetFromYaml("metallb.internal.secretsToCopy", "[]")
			f.ValuesSetFromYaml("metallb.internal.speakerNodeAffinity", "{}")
			f.HelmRender()
		})

		It("Should create a resources with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool1 := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool1")
			Expect(ipAddressPool1.Exists()).To(BeTrue())
			Expect(ipAddressPool1.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.0.0/24
`))

			ipAddressPool2 := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool2")
			Expect(ipAddressPool2.Exists()).To(BeTrue())
			Expect(ipAddressPool2.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.68.1.1-192.168.1.255
`))

			bgpPeer0 := f.KubernetesResource("BGPPeer", "d8-metallb", "bgp-peer-0")
			Expect(bgpPeer0.Exists()).To(BeTrue())
			Expect(bgpPeer0.Field("spec").String()).To(MatchYAML(`
holdTime: 3s
myASN: 64000
peerASN: 65000
peerAddress: 1.1.1.1
routerID: 10.0.0.254
`))

			bgpPeer1 := f.KubernetesResource("BGPPeer", "d8-metallb", "bgp-peer-1")
			Expect(bgpPeer1.Exists()).To(BeTrue())
			Expect(bgpPeer1.Field("spec").String()).To(MatchYAML(`
holdTime: 3s
myASN: 64000
nodeSelectors:
- matchLabels:
    node: metallb
peerASN: 65000
peerAddress: 1.1.1.2
`))
			bgpAdvertisement1 := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool1-0")
			Expect(bgpAdvertisement1.Exists()).To(BeTrue())
			Expect(bgpAdvertisement1.Field("spec").String()).To(MatchYAML(`
ipAddressPools:
- mypool1
`))

			bgpAdvertisement2 := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool2-0")
			Expect(bgpAdvertisement2.Exists()).To(BeTrue())
			Expect(bgpAdvertisement2.Field("spec").String()).To(MatchYAML(`
aggregationLength: 32
communities:
- 65535:65282
- 1111:1111
ipAddressPools:
- mypool2
localPref: 100
`))

			bgpAdvertisement3 := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool2-1")
			Expect(bgpAdvertisement3.Exists()).To(BeTrue())
			Expect(bgpAdvertisement3.Field("spec").String()).To(MatchYAML(`
aggregationLength: 32
ipAddressPools:
- mypool2
localPref: 150
`))

			l2Advertisement1 := f.KubernetesResource("L2Advertisement", "d8-metallb", "mypool1")
			Expect(l2Advertisement1.Exists()).To(BeFalse())

			l2Advertisement2 := f.KubernetesResource("L2Advertisement", "d8-metallb", "mypool2")
			Expect(l2Advertisement2.Exists()).To(BeFalse())

			bgpControllerDeployment := f.KubernetesResource("Deployment", "d8-metallb", "controller")
			Expect(bgpControllerDeployment.Exists()).To(BeTrue())
			bgpSpeakerDaemonset := f.KubernetesResource("DaemonSet", "d8-metallb", "speaker")
			Expect(bgpSpeakerDaemonset.Exists()).To(BeTrue())

		})
	})
})