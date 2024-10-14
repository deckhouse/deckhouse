/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

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

const (
	bgpPeers = `
- peer-address: 1.1.1.1
  peer-asn: 65000
  my-asn: 64000
  hold-time: 3s
- peer-address: 1.1.1.2
  peer-asn: 65000
  my-asn: 64000
  hold-time: 3s
  node-selector:
    matchLabels:
      node: "metallb"
`

	addressPoolsBGP = `
- name: mypool1
  protocol: bgp
  auto-assign: true
  avoid-buggy-ips: false
  addresses:
  - 192.168.0.0/24
- name: mypool2
  protocol: bgp
  addresses:
  - 192.68.1.1-192.168.1.255
  auto-assign: false
  avoid-buggy-ips: true
  bgp-advertisements:
  - aggregation-length: 32
    localpref: 100
    communities:
    - comm1
    - comm2
  - aggregation-length: 32
    localpref: 150
`

	addressPoolsL2 = `
- name: mypool1
  auto-assign: false
  avoid-buggy-ips: true
  protocol: layer2
  addresses:
  - 192.168.0.0/24
- name: mypool2
  auto-assign: true
  avoid-buggy-ips: false
  protocol: layer2
  addresses:
  - 192.168.1.0-192.168.1.255
`

	bgpCommunities = `
comm1: 65535:65282
comm2: 1111:1111
unusable: 2222:2222
`
)

var _ = Describe("Module :: metallb :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		f.ValuesSet("global.modulesImages.registry.base", "registryAddr")

		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
		f.ValuesSet("metallb.speaker.nodeSelector.mylabel", "myvalue")
	})

	Context("bgpPeers and addressPools in BGP mode are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("metallb.bgpPeers", bgpPeers)
			f.ValuesSetFromYaml("metallb.addressPools", addressPoolsBGP)
			f.ValuesSetFromYaml("metallb.bgpCommunities", bgpCommunities)
			f.HelmRender()
		})

		It("Should create a resources with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool1 := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool1")
			Expect(ipAddressPool1.Exists()).To(BeTrue())
			Expect(ipAddressPool1.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.0.0/24
autoAssign: true
avoidBuggyIPs: false
`))

			ipAddressPool2 := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool2")
			Expect(ipAddressPool2.Exists()).To(BeTrue())
			Expect(ipAddressPool2.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.68.1.1-192.168.1.255
autoAssign: false
avoidBuggyIPs: true
`))

			bgpPeer0 := f.KubernetesResource("BGPPeer", "d8-metallb", "bgp-peer-0")
			Expect(bgpPeer0.Exists()).To(BeTrue())
			Expect(bgpPeer0.Field("spec").String()).To(MatchYAML(`
holdTime: 3s
myASN: 64000
peerASN: 65000
peerAddress: 1.1.1.1
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

		})
	})
	Context("addressPools in L2 mode are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("metallb.addressPools", addressPoolsL2)
			f.HelmRender()
		})

		It("Should create a resources with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool1 := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool1")
			Expect(ipAddressPool1.Exists()).To(BeTrue())
			Expect(ipAddressPool1.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.0.0/24
autoAssign: false
avoidBuggyIPs: true
`))

			ipAddressPool2 := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool2")
			Expect(ipAddressPool2.Exists()).To(BeTrue())
			Expect(ipAddressPool2.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.1.0-192.168.1.255
autoAssign: true
avoidBuggyIPs: false
`))

			bgpAdvertisement1 := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool1-0")
			Expect(bgpAdvertisement1.Exists()).To(BeFalse())

			bgpAdvertisement2 := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool2-0")
			Expect(bgpAdvertisement2.Exists()).To(BeFalse())

			l2Advertisement1 := f.KubernetesResource("L2Advertisement", "d8-metallb", "mypool1")
			Expect(l2Advertisement1.Exists()).To(BeTrue())
			Expect(l2Advertisement1.Field("spec").String()).To(MatchYAML(`
ipAddressPools:
- mypool1
`))

			l2Advertisement2 := f.KubernetesResource("L2Advertisement", "d8-metallb", "mypool2")
			Expect(l2Advertisement2.Exists()).To(BeTrue())
			Expect(l2Advertisement2.Field("spec").String()).To(MatchYAML(`
ipAddressPools:
- mypool2
`))

		})
	})
})
