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
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modules:
    placement: {}
  discovery:
    kubernetesVersion: 1.23.5
    d8SpecificNodeCountByRole:
      worker: 3
      master: 3
`
	moduleValues = `
ntpServers: ["pool.ntp.org", "ntp.ubuntu.com"]
`
)

var _ = Describe("Module :: chrony :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("chrony", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-chrony")
			registrySecret := f.KubernetesResource("Secret", "d8-chrony", "deckhouse-registry")

			chronyDaemonSetTest := f.KubernetesResource("DaemonSet", "d8-chrony", "chrony")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(chronyDaemonSetTest.Exists()).To(BeTrue())
			Expect(chronyDaemonSetTest.Field("spec.template.spec.containers.0.env.0").String()).To(MatchJSON(`
  {
    "name": "NTP_SERVERS",
    "value": "pool.ntp.org. ntp.ubuntu.com."
  }
`))

		})
	})
})
