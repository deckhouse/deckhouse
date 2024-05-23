/*
Copyright 2024 Flant JSC
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
	l2LoadBalancerValues = `
loadBalancerClass: my-lb-class
internal:
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

var _ = Describe("Module :: l2LoadBalancer :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Cluster with l2LoadBalancer", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", "{}")
			f.ValuesSetFromYaml("l2LoadBalancer", l2LoadBalancerValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ipAddressPool := f.KubernetesResource("IPAddressPool", "d8-l2-load-balancer", "main-a")
			Expect(ipAddressPool.Exists()).To(BeTrue())
			Expect(ipAddressPool.Field("spec").String()).To(MatchYAML(`
addresses:
- 192.168.199.100-192.168.199.110
autoAssign: true
`))

			ipAdv := f.KubernetesResource("L2Advertisement", "d8-l2-load-balancer", "main-a")
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

			dsSpeaker := f.KubernetesResource("DaemonSet", "d8-l2-load-balancer", "speaker")
			Expect(dsSpeaker.Exists()).To(BeTrue())
		})
	})
})
