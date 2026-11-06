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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: deckhouse :: debug container image ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("names the debug container image of this release, from the registry images are rendered from", func() {
		configMap := f.KubernetesResource("ConfigMap", "d8-system", "debug-container")
		Expect(configMap.Exists()).To(BeTrue())
		Expect(configMap.Field("data.image").String()).To(Equal(
			f.ValuesGet("global.modulesImages.registry.base").String()+"@"+
				f.ValuesGet("global.modulesImages.digests.common.debugContainer").String()),
			"a digest pins the image to this release")
	})
})
