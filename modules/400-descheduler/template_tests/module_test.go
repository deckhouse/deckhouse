/*
Copyright 2022 Flant JSC

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

const globalValues = `
  enabledModules: ["vertical-pod-autoscaler-crd"]
  modules:
    placement: {}
  modulesImages:
    registry: registry.deckhouse.io/deckhouse/fe
    registryDockercfg: Y2ZnCg==
    tags:
      descheduler:
        descheduler: tagstring
  discovery:
    kubernetesVersion: 1.16.15
    d8SpecificNodeCountByRole:
      master: 42
`

var _ = Describe("Module :: monitoring-kubernetes-control-plane :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Defaults are applied on an empty CR", func() {
		BeforeEach(func() {
			moduleValues := `
internal:
  deschedulers:
  - metadata:
      name: test
    spec:
      deschedulerPolicy:
        strategies:
          highNodeUtilization:
            params:
              nodeResourceUtilizationThresholds:
                numberOfNodes: 2
`
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("descheduler", moduleValues)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			cm := f.KubernetesResource("ConfigMap", "d8-descheduler", "descheduler-policy-test")
			Expect(cm.Exists()).To(BeTrue())
		})
	})
})
