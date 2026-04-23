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
			Expect(corefile).To(MatchRegexp(`template IN AAAA \. \{\n\s*rcode NOERROR\n\s*\}`))
			Expect(corefile).To(MatchRegexp(`template IN PTR ip6\.arpa \{\n\s*rcode NXDOMAIN\n\s*\}`))
		})

	})
})
