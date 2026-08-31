/*
Copyright 2021 Flant JSC

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
	globalValues = `
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "cert-manager"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "1.32"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  d8SpecificNodeCountByRole:
    system: 1
modules:
  https:
    mode: CertManager
    certManager:
      clusterIssuerName: letsencrypt
  publicDomainTemplate: "%s.example.com"
  placement: {}
`
)

var _ = Describe("Module :: ciliumHubble :: helm template ::", func() {
	f := SetupHelmConfig(`{ciliumHubble: {internal: {deployDexAuthenticator: true, ui: {ca: CACA, key: ZXC, cert: CERT}, relay: {serverCerts: {ca: CACA, key: ZXC, cert: CERT}, clientCerts: {ca: CACA, key: ZXC, cert: CERT}}}, auth: {}}}`)

	Context("Cluster with ciliumHubble", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})
	})

	Context("Ingress and Gateway API enable overrides", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("ciliumHubble", `{internal: {deployDexAuthenticator: true, ui: {ca: CACA, key: ZXC, cert: CERT}, relay: {serverCerts: {ca: CACA, key: ZXC, cert: CERT}, clientCerts: {ca: CACA, key: ZXC, cert: CERT}}}, auth: {}}`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.clusterIsBootstrapped", true)
			f.ValuesSet("global.modules.gatewayAPI.gateway.name", "shared-gateway")
			f.ValuesSet("global.modules.gatewayAPI.gateway.namespace", "d8-alb")
		})

		It("disables Ingress and its certificate globally", func() {
			f.ValuesSet("global.modules.ingress.enabled", false)
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Ingress", "d8-cni-cilium", "hubble-ui").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Certificate", "d8-cni-cilium", "hubble").Exists()).To(BeFalse())
		})

		It("allows the module to enable Ingress when globally disabled", func() {
			f.ValuesSet("global.modules.ingress.enabled", false)
			f.ValuesSet("ciliumHubble.ingress.enabled", true)
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Ingress", "d8-cni-cilium", "hubble-ui").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Certificate", "d8-cni-cilium", "hubble").Exists()).To(BeTrue())
		})

		It("disables HTTPRoute and its certificate globally", func() {
			f.ValuesSet("global.modules.gatewayAPI.enabled", false)
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("HTTPRoute", "d8-cni-cilium", "hubble-ui").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Certificate", "d8-cni-cilium", "hubble-httproute").Exists()).To(BeFalse())
		})

		It("allows the module to enable HTTPRoute when globally disabled", func() {
			f.ValuesSet("global.modules.gatewayAPI.enabled", false)
			f.ValuesSet("ciliumHubble.gatewayAPI.enabled", true)
			f.HelmRender()

			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("HTTPRoute", "d8-cni-cilium", "hubble-ui").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Certificate", "d8-cni-cilium", "hubble-httproute").Exists()).To(BeTrue())
		})
	})
})
