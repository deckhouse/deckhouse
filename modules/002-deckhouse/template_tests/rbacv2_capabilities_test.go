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

// The capabilities of the deckhouse module in the RBACv2 role model. testing/rbacv2 holds them to the
// label contract, which decides the roles each one is aggregated into; these tests check what each one
// grants. A role that aggregates an edit capability aggregates the view one too (a manager is also a
// viewer), which is why the edit capabilities carry no reading verbs of their own.

const (
	deckhouseUseViewCapability    = "d8:namespace-capability:deckhouse:view"
	deckhouseUseEditCapability    = "d8:namespace-capability:deckhouse:edit"
	deckhouseManageViewCapability = "d8:system-capability:deckhouse:view"
	deckhouseManageEditCapability = "d8:system-capability:deckhouse:edit"
)

// deckhouseConfigurationResources are the deckhouse.io resources a manager of the deckhouse module
// edits: the modules and their configuration, the releases and the license, and the package
// repositories with the packages they bring.
var deckhouseConfigurationResources = []string{
	"clusterlicenses",
	"deckhousereleases",
	"moduleconfigs",
	"modulepulloverrides",
	"modulereleases",
	"modules",
	"modulesources",
	"moduleupdatepolicies",
	"packagerepositories",
	"packagerepositoryoperations",
	"applicationpackageversions",
	"applicationpackages",
}

// deckhouseGrants is the grants of the given verbs on deckhouse.io resources, in the form of roleGrants.
func deckhouseGrants(verbs []string, resources ...string) map[string][]string {
	grants := map[string][]string{}
	for _, resource := range resources {
		grants["deckhouse.io/"+resource] = verbs
	}
	return grants
}

var _ = Describe("Module :: deckhouse :: RBACv2 capabilities ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	capability := func(name string) map[string][]string {
		return roleGrants(f.KubernetesGlobalResource("ClusterRole", name))
	}

	It("lets a namespace viewer read the applications and their packages", func() {
		Expect(capability(deckhouseUseViewCapability)).To(Equal(
			deckhouseGrants(rbacReadVerbs, "applications", "applicationpackages", "applicationpackageversions")))
	})

	It("lets a namespace manager install and remove applications, and nothing else", func() {
		Expect(capability(deckhouseUseEditCapability)).To(Equal(deckhouseGrants(rbacWriteVerbs, "applications")),
			"the packages are cluster-wide and are not edited from a namespace")
	})

	It("lets a system viewer read the configuration, the license state and the module settings definitions", func() {
		resources := append([]string{"effectivelicenses", "modulesettingsdefinitions"}, deckhouseConfigurationResources...)
		Expect(capability(deckhouseManageViewCapability)).To(Equal(deckhouseGrants(rbacReadVerbs, resources...)))
	})

	It("lets a system manager edit the configuration, but not the license state or the module settings definitions", func() {
		grants := capability(deckhouseManageEditCapability)
		Expect(grants).To(Equal(deckhouseGrants(rbacAllVerbs, deckhouseConfigurationResources...)))
		Expect(grants).ToNot(HaveKey("deckhouse.io/effectivelicenses"),
			"only Deckhouse writes EffectiveLicense, see licensing.deckhouse.io in validation.yaml")
		Expect(grants).ToNot(HaveKey("deckhouse.io/modulesettingsdefinitions"),
			"Deckhouse writes the settings definitions from the modules")
	})
})
