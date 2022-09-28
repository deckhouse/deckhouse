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
        - node: metallb`

	addressPoolsBGP = `
    - name: mypool
      protocol: bgp
      addresses:
      - 192.168.0.0/24`

	addressPoolsL2 = `
    - name: mypool
      protocol: layer2
      addresses:
      - 192.168.0.0/24`
)

var _ = Describe("Module :: metallb :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.discovery.kubernetesVersion", "1.23.5")
		f.ValuesSet("global.modulesImages.registry", "registryAddr")

		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
		f.ValuesSet("metallb.speaker.nodeSelector.mylabel", "myvalue")
	})

	Context("bgpPeers and addressPools in BGP mode are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("metallb.bgpPeers", bgpPeers)
			f.ValuesSetFromYaml("metallb.addressPools", addressPoolsBGP)
			f.HelmRender()
		})

		It("Should create a resources with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool")
			Expect(ipAddressPool.Exists()).To(BeTrue())

			Expect(ipAddressPool.Field("spec.addresses").String()).To(MatchYAML(`
- 192.168.0.0/24
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
  - node: metallb
peerASN: 65000
peerAddress: 1.1.1.2
`))
			bgpAdvertisement := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool")
			Expect(bgpAdvertisement.Exists()).To(BeTrue())

			l2Advertisement := f.KubernetesResource("L2Advertisement", "d8-metallb", "mypool")
			Expect(l2Advertisement.Exists()).To(BeFalse())

		})
	})
	Context("addressPools in L2 mode are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("metallb.addressPools", addressPoolsL2)
			f.HelmRender()
		})

		It("Should create a resources with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool := f.KubernetesResource("IPAddressPool", "d8-metallb", "mypool")
			Expect(ipAddressPool.Exists()).To(BeTrue())

			Expect(ipAddressPool.Field("spec.addresses").String()).To(MatchYAML(`
- 192.168.0.0/24
`))

			bgpAdvertisement := f.KubernetesResource("BGPAdvertisement", "d8-metallb", "mypool")
			Expect(bgpAdvertisement.Exists()).To(BeFalse())

			l2Advertisement := f.KubernetesResource("L2Advertisement", "d8-metallb", "mypool")
			Expect(l2Advertisement.Exists()).To(BeTrue())

		})
	})
})
