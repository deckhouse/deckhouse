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

// The PackageRepository Module v2 takes the modules of the Deckhouse registry from. It is the
// counterpart of the ModuleSource deckhouse, so it names the same repository: the address the images
// are rendered from, which GetModulesImages sets apart from address and path.

const defaultPackageRepository = "deckhouse-modules"

var _ = Describe("Module :: deckhouse :: default package repository ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	render := func(enableModuleV2 bool) {
		f.ValuesSet("deckhouse.enableModuleV2", enableModuleV2)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	It("is not rendered without Module v2", func() {
		render(false)

		Expect(f.KubernetesGlobalResource("PackageRepository", defaultPackageRepository).Exists()).To(BeFalse())
	})

	It("points Module v2 at the modules of the Deckhouse registry", func() {
		render(true)

		repository := f.KubernetesGlobalResource("PackageRepository", defaultPackageRepository)
		Expect(repository.Exists()).To(BeTrue())
		Expect(repository.Field("spec.registry.repo").String()).To(Equal("registry.example.com/modules"),
			"the repository images are rendered from, the same as the ModuleSource deckhouse names")
		Expect(repository.Field("spec.registry.repo").String()).
			To(Equal(f.KubernetesGlobalResource("ModuleSource", "deckhouse").Field("spec.registry.repo").String()))
		Expect(repository.Field("spec.registry.dockerCfg").String()).To(Equal(f.ValuesGet("global.modulesImages.registry.dockercfg").String()))
		Expect(repository.Field("spec.registry.scheme").String()).To(Equal("HTTPS"))
		Expect(repository.Field("spec.scanInterval").String()).To(Equal("6h"))
	})

	It("passes the registry CA when it is set", func() {
		render(true)

		repository := f.KubernetesGlobalResource("PackageRepository", defaultPackageRepository)
		Expect(repository.Field("spec.registry.ca").String()).To(Equal(f.ValuesGet("global.modulesImages.registry.CA").String() + "\n"))
	})

	It("passes no CA when the registry has none", func() {
		f.ValuesSet("global.modulesImages.registry.CA", "")
		render(true)

		repository := f.KubernetesGlobalResource("PackageRepository", defaultPackageRepository)
		Expect(repository.Exists()).To(BeTrue())
		Expect(repository.Field("spec.registry.ca").Exists()).To(BeFalse())
	})
})
