/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: node-local-dns :: helm template", func() {
	hec := SetupHelmConfig(`{"nodeLocalDns":{}}`)

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
	})

	Context("Test helm render", func() {

		It("Should successful render helm", func() {
			hec.HelmRender()
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
		})

		It("Should not render IPv6 disabling templates by default", func() {
			hec.HelmRender()
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			configMap := hec.KubernetesResource("ConfigMap", "kube-system", "node-local-dns")
			Expect(configMap.Exists()).To(BeTrue())

			corefile := configMap.Field("data.Corefile").String()
			Expect(strings.Contains(corefile, "template IN AAAA .")).To(BeFalse())
			Expect(strings.Contains(corefile, "template IN PTR ip6.arpa")).To(BeFalse())
		})

		It("Should render IPv6 disabling templates when enabled", func() {
			hec.ValuesSet("nodeLocalDns.disableIPv6", true)
			hec.HelmRender()
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			configMap := hec.KubernetesResource("ConfigMap", "kube-system", "node-local-dns")
			Expect(configMap.Exists()).To(BeTrue())

			corefile := configMap.Field("data.Corefile").String()
			Expect(strings.Contains(corefile, "template IN AAAA . {\n          rcode NOERROR\n      }")).To(BeTrue())
			Expect(strings.Contains(corefile, "template IN PTR ip6.arpa {\n          rcode NXDOMAIN\n      }")).To(BeTrue())
		})

		It("Should bind coredns to IPv4 only with cilium when IPv6 is disabled", func() {
			hec.ValuesSetFromYaml("global.enabledModules", `["cni-cilium"]`)
			hec.ValuesSet("nodeLocalDns.disableIPv6", true)
			hec.HelmRender()
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			configMap := hec.KubernetesResource("ConfigMap", "kube-system", "node-local-dns")
			Expect(configMap.Exists()).To(BeTrue())

			corefile := configMap.Field("data.Corefile").String()
			Expect(strings.Contains(corefile, "bind 0.0.0.0")).To(BeTrue())
		})

	})
})
