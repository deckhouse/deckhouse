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
	"slices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The roles of user-authz-cluster-roles.yaml extend the user-authz access levels by an annotation, and
// a level holds the rules of the roles annotated with it and with every level below it: user-authz
// aggregates them into user-authz:<level>:custom
// (modules/140-user-authz/templates/cluster-roles-custom-aggregated.yaml). What a level may do is
// therefore the union of the roles up to it, and that union is what these tests check.

// userAuthzAccessLevels are the access levels a role can extend, from the narrowest.
var userAuthzAccessLevels = []string{"User", "PrivilegedUser", "Editor", "Admin", "ClusterEditor", "ClusterAdmin"}

// userAuthzDeckhouseRoles are the ClusterRoles of the template by the access level each one extends.
var userAuthzDeckhouseRoles = map[string]string{
	"User":         "d8:user-authz:deckhouse:user",
	"Admin":        "d8:user-authz:deckhouse:admin",
	"ClusterAdmin": "d8:user-authz:deckhouse:cluster-admin",
}

// The verbs a role grants, sorted the way roleGrants returns them.
var (
	rbacReadVerbs  = []string{"get", "list", "watch"}
	rbacWriteVerbs = []string{"create", "delete", "deletecollection", "patch", "update"}
	rbacAllVerbs   = []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"}
)

// roleGrants flattens the rules of a role into what it allows: "<group>/<resource>", or "<resource>" for
// the core group, mapped to the sorted verbs, so that a test compares what a role allows however its
// rules are split and ordered. A rule with resourceNames allows its verbs on those objects only, so it
// is kept apart under "<group>/<resource>:<name>".
func roleGrants(role object_store.KubeObject) map[string][]string {
	Expect(role.Exists()).To(BeTrue(), "the role should be rendered")

	grants := map[string][]string{}
	for _, rule := range role.Field("rules").Array() {
		var resourceNames []string
		for _, name := range rule.Get("resourceNames").Array() {
			resourceNames = append(resourceNames, name.String())
		}
		for _, group := range rule.Get("apiGroups").Array() {
			for _, resource := range rule.Get("resources").Array() {
				for _, key := range ruleKeys(group.String(), resource.String(), resourceNames) {
					for _, verb := range rule.Get("verbs").Array() {
						grants[key] = append(grants[key], verb.String())
					}
				}
			}
		}
	}

	for key, verbs := range grants {
		slices.Sort(verbs)
		grants[key] = slices.Compact(verbs)
	}
	return grants
}

// ruleKeys names what a rule grants on one resource of one group, in the form of roleGrants.
func ruleKeys(group, resource string, resourceNames []string) []string {
	key := resource
	if group != "" {
		key = group + "/" + resource
	}
	if len(resourceNames) == 0 {
		return []string{key}
	}

	keys := make([]string, 0, len(resourceNames))
	for _, name := range resourceNames {
		keys = append(keys, key+":"+name)
	}
	return keys
}

// mergeGrants adds the grants of from to into.
func mergeGrants(into, from map[string][]string) {
	for key, verbs := range from {
		merged := append(slices.Clone(into[key]), verbs...)
		slices.Sort(merged)
		into[key] = slices.Compact(merged)
	}
}

var _ = Describe("Module :: deckhouse :: user-authz access levels ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	// levelGrants is what an access level may do through the roles of this module.
	levelGrants := func(level string) map[string][]string {
		Expect(userAuthzAccessLevels).To(ContainElement(level))

		grants := map[string][]string{}
		for _, below := range userAuthzAccessLevels[:slices.Index(userAuthzAccessLevels, level)+1] {
			name, ok := userAuthzDeckhouseRoles[below]
			if !ok {
				continue
			}
			role := f.KubernetesGlobalResource("ClusterRole", name)
			Expect(role.Exists()).To(BeTrue(), "the %s role should be rendered", name)
			Expect(role.Field(`metadata.annotations.user-authz\.deckhouse\.io/access-level`).String()).To(Equal(below),
				"the annotation is what puts %s at the %s level", name, below)
			mergeGrants(grants, roleGrants(role))
		}
		return grants
	}

	It("lets a User read the applications and their packages, and nothing else", func() {
		Expect(levelGrants("User")).To(Equal(map[string][]string{
			"deckhouse.io/applications":               rbacReadVerbs,
			"deckhouse.io/applicationpackages":        rbacReadVerbs,
			"deckhouse.io/applicationpackageversions": rbacReadVerbs,
		}), "the package repositories and their operations are not for a User")
	})

	It("lets an Admin install and remove applications, and only read their packages", func() {
		Expect(levelGrants("Admin")).To(Equal(map[string][]string{
			"deckhouse.io/applications":               rbacAllVerbs,
			"deckhouse.io/applicationpackages":        rbacReadVerbs,
			"deckhouse.io/applicationpackageversions": rbacReadVerbs,
		}), "the reading verbs of an Admin come from the User role")
	})

	It("lets a ClusterAdmin manage the applications, their packages and the package repositories", func() {
		grants := levelGrants("ClusterAdmin")
		for _, resource := range []string{
			"applications",
			"applicationpackages",
			"applicationpackageversions",
			"packagerepositories",
			"packagerepositoryoperations",
		} {
			Expect(grants).To(HaveKeyWithValue("deckhouse.io/"+resource, rbacAllVerbs), "a ClusterAdmin on %s", resource)
		}
	})
})
