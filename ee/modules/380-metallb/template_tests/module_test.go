/*
Copyright 2021 Flant CJSC
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
      hold-time: 3s`

	addressPools = `
    - name: mypool
      protocol: bgp
      addresses:
      - 192.168.0.0/24`
)

var _ = Describe("Module :: metallb :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		f.ValuesSet("global.modulesImages.registry", "registryAddr")

		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
		f.ValuesSet("metallb.speaker.nodeSelector.mylabel", "myvalue")
	})

	Context("bgpPeers and addressPools are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("metallb.bgpPeers", bgpPeers)
			f.ValuesSetFromYaml("metallb.addressPools", addressPools)
			f.HelmRender()
		})

		It("Should create a ConfigMap `config` with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			crb := f.KubernetesResource("ConfigMap", "d8-metallb", "config")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("data.config").String()).To(MatchYAML(`
peers:
- hold-time: 3s
  my-asn: 64000
  peer-address: 1.1.1.1
  peer-asn: 65000
- hold-time: 3s
  my-asn: 64000
  peer-address: 1.1.1.2
  peer-asn: 65000
address-pools:
- addresses:
  - 192.168.0.0/24
  name: mypool
  protocol: bgp
`))
		})

	})

})
