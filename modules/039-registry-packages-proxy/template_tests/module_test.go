/*
Copyright 2026 Flant JSC

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
enabledModules: ["vertical-pod-autoscaler"]
modules:
  placement: {}
discovery:
  kubernetesVersion: 1.31.8
  d8SpecificNodeCountByRole:
    master: 3
`

var _ = Describe("Module :: registry-packages-proxy :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("HA disabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", false)
			f.HelmRender()
		})

		It("PDB does not exist", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-instance-manager", "registry-packages-proxy")
			Expect(pdb.Exists()).To(BeFalse())
		})
	})

	Context("HA enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("PDB exists", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-instance-manager", "registry-packages-proxy")
			Expect(pdb.Exists()).To(BeTrue())
		})
	})
})
