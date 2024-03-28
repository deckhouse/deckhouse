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
	addressPoolsL2 = `
- name: mypool1
  avoid-buggy-ips: true
  addresses:
  - 192.168.0.0/24
- name: mypool2
  avoid-buggy-ips: false
  addresses:
  - 192.168.1.0-192.168.1.255
`
)

var _ = Describe("Module :: l2-load-balancer :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		f.ValuesSet("global.modulesImages.registry.base", "registryAddr")

		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
	})

	Context("addressPools in L2 mode are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("l2LoadBalancer.addressPools", addressPoolsL2)
			f.ValuesSetFromYaml("l2LoadBalancer.internal.l2LoadBalancers", `
- name: test
  namespace: test
  labelSelector:
    app: test
  nodes:
  - name: front-1
  - name: front-2
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 8080
`)
			f.HelmRender()
		})

		It("Should create a resources with our values", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool1 := f.KubernetesResource("IPAddressPool", "d8-l2-load-balancer", "mypool1")
			Expect(ipAddressPool1.Exists()).To(BeTrue())
			Expect(ipAddressPool1.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.0.0/24
autoAssign: false
avoidBuggyIPs: true
`))

			ipAddressPool2 := f.KubernetesResource("IPAddressPool", "d8-l2-load-balancer", "mypool2")
			Expect(ipAddressPool2.Exists()).To(BeTrue())
			Expect(ipAddressPool2.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.1.0-192.168.1.255
autoAssign: false
avoidBuggyIPs: false
`))

			l2Advertisement1 := f.KubernetesResource("L2Advertisement", "d8-l2-load-balancer", "mypool1")
			Expect(l2Advertisement1.Exists()).To(BeTrue())
			Expect(l2Advertisement1.Field("spec").String()).To(MatchYAML(`
ipAddressPools:
- mypool1
`))

			l2Advertisement2 := f.KubernetesResource("L2Advertisement", "d8-l2-load-balancer", "mypool2")
			Expect(l2Advertisement2.Exists()).To(BeTrue())
			Expect(l2Advertisement2.Field("spec").String()).To(MatchYAML(`
ipAddressPools:
- mypool2
`))

			l2Service1 := f.KubernetesResource("Service", "test", "d8-l2-load-balancer-test-0")
			Expect(l2Service1.Exists()).To(BeTrue())
			Expect(l2Service1.Field("spec").String()).To(MatchYAML(`
ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 8080
selector:
  app: test
externalTrafficPolicy: Local
sessionAffinity: None
type: LoadBalancer
loadBalancerClass: l2-load-balancer.network.deckhouse.io
`))

		})
	})
})
