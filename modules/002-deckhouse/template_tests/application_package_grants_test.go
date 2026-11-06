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

// The ApplicationPackage grant registration consists of objects of the multitenancy-manager CRDs, so
// these tests check that the deckhouse release carries them exactly while that module is enabled and
// its CRDs are served, and that the catalog never shows the installed applications of other projects.

const (
	grantableDefinitionAPI = "multitenancy.deckhouse.io/v1alpha1/GrantableClusterResourceDefinition"
	grantableReferenceAPI  = "multitenancy.deckhouse.io/v1alpha1/GrantableClusterResourceReference"

	applicationPackagesGrant     = "applicationpackages"
	applicationPackagesReference = "applicationpackages-application"
)

var _ = Describe("Module :: deckhouse :: application package grants ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	render := func(enabledModules string, apiVersions ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSetFromYaml("global.enabledModules", enabledModules)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender(WithAPIVersions(apiVersions...))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	rendered := func() (bool, bool) {
		return f.KubernetesGlobalResource("GrantableClusterResourceDefinition", applicationPackagesGrant).Exists(),
			f.KubernetesGlobalResource("GrantableClusterResourceReference", applicationPackagesReference).Exists()
	}

	Context("multitenancy-manager is enabled and its CRDs are served", func() {
		BeforeEach(func() {
			render(`["multitenancy-manager"]`, grantableDefinitionAPI, grantableReferenceAPI)
		})

		It("registers ApplicationPackage with every package available by default", func() {
			definition := f.KubernetesGlobalResource("GrantableClusterResourceDefinition", applicationPackagesGrant)
			Expect(definition.Exists()).To(BeTrue())
			Expect(definition.Field("spec.grantedResource").String()).To(MatchJSON(`{"apiGroup":"deckhouse.io","kind":"ApplicationPackage"}`))
			Expect(definition.Field("spec.enforcement").String()).To(Equal("Managed"))
			Expect(definition.Field("spec.defaultAvailability").String()).To(Equal("All"),
				"enabling multitenancy-manager must not take packages away from projects")
		})

		It("shows the repositories and the channel versions, never the installed applications", func() {
			definition := f.KubernetesGlobalResource("GrantableClusterResourceDefinition", applicationPackagesGrant)
			Expect(definition.Field("spec.catalogFields").String()).To(MatchJSON(`[
				{"name":"availableRepositories","path":"$.status.availableRepositories"},
				{"name":"releaseChannels","path":"$.status.releaseChannels"}
			]`), "status.usedBy names the applications and the namespaces of every project")
		})

		It("checks the package of an Application against the registration", func() {
			reference := f.KubernetesGlobalResource("GrantableClusterResourceReference", applicationPackagesReference)
			Expect(reference.Exists()).To(BeTrue())
			Expect(reference.Field("spec.grantableClusterResourceName").String()).To(Equal(applicationPackagesGrant))
			Expect(reference.Field("spec.rule").String()).To(MatchJSON(`{"apiGroups":["deckhouse.io"],"apiVersions":["*"],"resources":["applications"]}`))
			Expect(reference.Field("spec.fieldPaths").String()).To(MatchJSON(`[{"path":"$.spec.packageName","defaulting":"None"}]`))
		})
	})

	It("renders nothing while multitenancy-manager is disabled, even with its CRDs left in the cluster", func() {
		render(`["operator-prometheus"]`, grantableDefinitionAPI, grantableReferenceAPI)

		definition, reference := rendered()
		Expect(definition).To(BeFalse())
		Expect(reference).To(BeFalse())
	})

	It("renders nothing while the multitenancy-manager CRDs are not served yet", func() {
		render(`["multitenancy-manager"]`)

		definition, reference := rendered()
		Expect(definition).To(BeFalse())
		Expect(reference).To(BeFalse())
	})
})
