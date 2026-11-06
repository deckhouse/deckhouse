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
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The PriorityClasses of priority-class/, which the pods of the cluster are scheduled with. Their
// values decide which pods are preempted first, and the API server refuses to change the value of an
// existing PriorityClass, so a changed value would fail the release on every cluster that has it.

var deckhousePriorityClasses = map[string]int64{
	"production-high":      9000,
	"cluster-medium":       7000,
	"production-medium":    6000,
	"deployment-machinery": 5000,
	"production-low":       4000,
	"staging":              3000,
	"cluster-low":          2000,
	"develop":              1000,
	"standby":              -1,
}

var _ = Describe("Module :: deckhouse :: priority classes ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("keeps the value of every class", func() {
		templates, err := filepath.Glob("../templates/priority-class/*.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(templates).To(HaveLen(len(deckhousePriorityClasses)), "every class the module renders should be listed here")

		for name, value := range deckhousePriorityClasses {
			class := f.KubernetesGlobalResource("PriorityClass", name)
			Expect(class.Exists()).To(BeTrue(), "the %s PriorityClass should be rendered", name)
			Expect(class.Field("value").Int()).To(Equal(value), "the value of %s cannot be changed in a cluster", name)
		}
	})

	It("gives develop to the pods that name no class, and makes no other class the default", func() {
		for name := range deckhousePriorityClasses {
			class := f.KubernetesGlobalResource("PriorityClass", name)
			Expect(class.Field("globalDefault").Bool()).To(Equal(name == "develop"),
				"the API server admits one default PriorityClass, and develop is the one of this module: %s", name)
		}
	})
})
