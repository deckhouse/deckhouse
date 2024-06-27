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
enabledModules: ["vertical-pod-autoscaler-crd", "prometheus", "operator-prometheus-crd"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.28.10"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
  proxy:
    httpProxy: "http://1.2.3.4:80"
    httpsProxy: "https://1.2.3.4:443"
    noProxy:
    - example.com
`

	moduleValuesForMasterNode = `
bundle: Default
logLevel: Info
internal:
  webhookHandlerCert:
    crt: a
    key: b
    ca: c
  admissionWebhookCert:
    crt: a
    key: b
    ca: c
  currentReleaseImageName: test
`

	moduleValuesForDeckhouseNode = `
bundle: Default
logLevel: Info
nodeSelector:
  node-role.kubernetes.io/deckhouse: ""
tolerations:
- key: testkey
  operator: Exists
internal:
  webhookHandlerCert:
    crt: a
    key: b
    ca: c
  admissionWebhookCert:
    crt: a
    key: b
    ca: c
  currentReleaseImageName: test
`
)

var _ = Describe("Module :: deckhouse :: helm template ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/control-plane: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
  - operator: Exists
`))
		})
	})

	Context("Cluster with deckhouse on system node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/deckhouse: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: testkey
  operator: Exists
`))
		})
	})

})
