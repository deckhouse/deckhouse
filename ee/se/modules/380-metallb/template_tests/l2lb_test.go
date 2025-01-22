/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
	addressPoolsLayer2 = `
- addresses:
  - 192.168.227.100-92.168.227.200
  name: mypool
  protocol: layer2
`

	l2LoadBalancerValues = `
l2loadbalancers:
  - name: main-a
    interfaces:
    - eth0
    - eth1
    addressPool:
    - 192.168.199.100-192.168.199.110
    nodeSelector:
      node-role.kubernetes.io/worker: ""
l2lbservices:
- name: ingress-1
  namespace: nginx
  preferredNode: frontend-1
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app: nginx
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

	Context("Cluster with l2LoadBalancer", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
			f.ValuesSetFromYaml("metallb.loadBalancerClass", "my-lb-class")
			f.ValuesSetFromYaml("metallb.internal", l2LoadBalancerValues)
			f.ValuesSetFromYaml("metallb.addressPools", addressPoolsLayer2)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool := f.KubernetesResource("IPAddressPool", "d8-metallb", "main-a")
			Expect(ipAddressPool.Exists()).To(BeTrue())
			Expect(ipAddressPool.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.199.100-192.168.199.110
autoAssign: true
`))

			ipAdv := f.KubernetesResource("L2Advertisement", "d8-metallb", "main-a")
			Expect(ipAdv.Exists()).To(BeTrue())
			Expect(ipAdv.Field("spec").String()).To(MatchYAML(`
interfaces:
- eth0
- eth1
ipAddressPools:
- main-a
nodeSelectors:
- matchLabels:
    node-role.kubernetes.io/worker: ""
`))

			dsSpeaker := f.KubernetesResource("DaemonSet", "d8-metallb", "l2lb-speaker")
			Expect(dsSpeaker.Exists()).To(BeTrue())
		})
	})
})
