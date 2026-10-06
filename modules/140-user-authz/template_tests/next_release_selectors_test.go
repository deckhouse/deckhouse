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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/flant/kube-client/manifest/releaseutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	rbacKindLabel = "rbac.deckhouse.io/kind"
	moduleLabel   = "module"
)

// nextReleaseCapability is an entry of testdata/next_release_capabilities.yaml, a capability that a
// module renders in the next release, with the old roles whose aliases collect it there.
type nextReleaseCapability struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	CollectedBy []string          `json:"collectedBy"`
	External    bool              `json:"external"`
}

// The next release relabels the capabilities of the modules, and user-authz can converge minutes
// after the other modules. The old roles of this release stay in place until then, so they must
// already collect the relabelled capabilities the way their aliases will.
var _ = Describe("Module :: user-authz :: helm template :: next release selectors", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	fixturePath := filepath.Join(filepath.Dir(thisFile), "testdata", "next_release_capabilities.yaml")

	var (
		roles        map[string]rbacv1.ClusterRole
		oldRoles     []string
		capabilities []nextReleaseCapability
	)

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("global.enabledModules", `[]`)
		f.ValuesSet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", "test")

		f.ValuesSet("userAuthz.enableMultiTenancy", false)
		f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", true)
		f.ValuesSetFromYaml("userAuthz.internal.clusterAuthRuleCrds", `[]`)
		f.ValuesSetFromYaml("userAuthz.internal.authRuleCrds", `[]`)
		f.ValuesSet("userAuthz.internal.webhookCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.key", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.key", "test")

		rendered := map[string]string{}
		f.HelmRender(WithRenderOutput(rendered))
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		roles = renderedClusterRoles(rendered)
		oldRoles = aliasedRoleNames(roles)
		capabilities = loadNextReleaseCapabilities(fixturePath)
	})

	It("Should know every old role the aliases replace", func() {
		aliased := sets.New[string]()
		for _, capability := range capabilities {
			aliased.Insert(capability.CollectedBy...)
		}

		Expect(oldRoles).To(HaveLen(24))
		Expect(oldRoles).To(Equal(sets.List(aliased)),
			"the fixture must name exactly the old roles of this release")
	})

	It("Should keep the selectors of this scheme", func() {
		for _, name := range oldRoles {
			Expect(roles[name].AggregationRule.ClusterRoleSelectors).To(
				ContainElement(metav1.LabelSelector{MatchLabels: ownSchemeLabels(name)}),
				"%s must still select the labels of this scheme", name)
		}
	})

	It("Should collect the capabilities of the modules of this repository the way their aliases do", func() {
		universe := withCapabilities(roles, capabilities)

		for _, name := range oldRoles {
			expected := sets.New[string]()
			for _, capability := range capabilities {
				if !capability.External && slices.Contains(capability.CollectedBy, name) {
					expected.Insert(capability.Name)
				}
			}

			collected := collectedCapabilities(name, universe).Intersection(capabilityNames(capabilities))
			Expect(sets.List(collected)).To(Equal(sets.List(expected)), "%s", name)
		}
	})

	It("Should leave the capabilities of modules installed from a module source to the aliases", func() {
		universe := withCapabilities(roles, capabilities)

		external := sets.New[string]()
		for _, capability := range capabilities {
			if capability.External {
				Expect(capability.CollectedBy).ToNot(BeEmpty(), "%s should be one an alias collects", capability.Name)
				external.Insert(capability.Name)
			}
		}
		Expect(external).ToNot(BeEmpty())

		for _, name := range oldRoles {
			collected := collectedCapabilities(name, universe).Intersection(external)
			Expect(sets.List(collected)).To(BeEmpty(), "%s must grant on this release what it granted before", name)
		}
	})

	It("Should take by the new labels only the capabilities of the modules of this repository", func() {
		inTree := sets.New[string]()
		for _, capability := range capabilities {
			if !capability.External {
				inTree.Insert(capability.Labels[moduleLabel])
			}
		}

		for name, role := range roles {
			Expect(role.Labels[rbacKindLabel]).ToNot(Equal("capability"), "%s", name)
		}

		for _, name := range oldRoles {
			for _, selector := range roles[name].AggregationRule.ClusterRoleSelectors {
				kind := selector.MatchLabels[rbacKindLabel]
				if kind == "manage" || kind == "use" {
					continue
				}

				Expect(kind).To(Equal("capability"), "%s: %v", name, selector)
				modules := selectorModules(selector)
				Expect(modules).ToNot(BeEmpty(), "%s: %v must require a module", name, selector)
				Expect(inTree.HasAll(modules...)).To(BeTrue(),
					"%s: %v must name only modules of this repository, have %v", name, selector, sets.List(inTree))
			}
		}
	})
})

// ownSchemeLabels returns the matchLabels an old role selects in this scheme.
func ownSchemeLabels(name string) map[string]string {
	parts := strings.Split(name, ":")
	switch {
	case parts[1] == "manage":
		return map[string]string{
			rbacKindLabel: "manage",
			fmt.Sprintf("rbac.deckhouse.io/aggregate-to-%s-as", parts[2]): parts[3],
		}
	case len(parts) == 5:
		return map[string]string{rbacKindLabel: "use", "rbac.deckhouse.io/aggregate-to-kubernetes-as": parts[3]}
	default:
		return map[string]string{rbacKindLabel: "use", "rbac.deckhouse.io/aggregate-to-all-as": parts[3]}
	}
}

// renderedClusterRoles parses the ClusterRoles of every rendered template.
func renderedClusterRoles(rendered map[string]string) map[string]rbacv1.ClusterRole {
	roles := map[string]rbacv1.ClusterRole{}

	for _, manifests := range rendered {
		for _, manifest := range releaseutil.SplitManifests(manifests) {
			object := ManifestStringToUnstructed(manifest)
			if object == nil || object.GetKind() != "ClusterRole" {
				continue
			}

			var role rbacv1.ClusterRole
			err := k8sruntime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &role)
			ExpectWithOffset(1, err).ShouldNot(HaveOccurred())
			roles[role.Name] = role
		}
	}

	return roles
}

// aliasedRoleNames returns the sorted names of the aggregated d8:manage:* and d8:use:role:* roles,
// which the next release turns into aliases.
func aliasedRoleNames(roles map[string]rbacv1.ClusterRole) []string {
	names := make([]string, 0)

	for name, role := range roles {
		isOldRole := strings.HasPrefix(name, "d8:manage:") || strings.HasPrefix(name, "d8:use:role:")
		if isOldRole && role.AggregationRule != nil {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	return names
}

func loadNextReleaseCapabilities(path string) []nextReleaseCapability {
	content, err := os.ReadFile(path)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	var capabilities []nextReleaseCapability
	ExpectWithOffset(1, yaml.UnmarshalStrict(content, &capabilities)).To(Succeed())
	ExpectWithOffset(1, capabilities).ToNot(BeEmpty())

	return capabilities
}

func capabilityNames(capabilities []nextReleaseCapability) sets.Set[string] {
	names := sets.New[string]()
	for _, capability := range capabilities {
		names.Insert(capability.Name)
	}

	return names
}

// withCapabilities returns the rendered ClusterRoles together with the capabilities of the fixture,
// the objects the aggregation controller sees while the other modules already run the next release.
func withCapabilities(roles map[string]rbacv1.ClusterRole, capabilities []nextReleaseCapability) map[string]rbacv1.ClusterRole {
	universe := make(map[string]rbacv1.ClusterRole, len(roles)+len(capabilities))
	for name, role := range roles {
		universe[name] = role
	}

	for _, capability := range capabilities {
		ExpectWithOffset(1, universe).ToNot(HaveKey(capability.Name), "a capability must not shadow a rendered role")
		universe[capability.Name] = rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: capability.Name, Labels: capability.Labels},
		}
	}

	return universe
}

// collectedCapabilities returns the names of the ClusterRoles without an aggregation rule whose rules
// the aggregation of the named role takes, directly or through the aggregated roles it selects.
func collectedCapabilities(name string, roles map[string]rbacv1.ClusterRole) sets.Set[string] {
	collected := sets.New[string]()
	visited := sets.New[string](name)
	pending := []string{name}

	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		for _, selected := range selectedRoles(roles[current], roles) {
			if visited.Has(selected) {
				continue
			}
			visited.Insert(selected)

			if roles[selected].AggregationRule == nil {
				collected.Insert(selected)
				continue
			}
			pending = append(pending, selected)
		}
	}

	return collected
}

// selectedRoles returns the names of the ClusterRoles that a selector of the aggregation rule of
// role matches, with the label selector semantics of the aggregation controller.
func selectedRoles(role rbacv1.ClusterRole, roles map[string]rbacv1.ClusterRole) []string {
	if role.AggregationRule == nil {
		return nil
	}

	selectors := make([]labels.Selector, 0, len(role.AggregationRule.ClusterRoleSelectors))
	for _, clusterRoleSelector := range role.AggregationRule.ClusterRoleSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&clusterRoleSelector)
		ExpectWithOffset(2, err).ShouldNot(HaveOccurred(), "%s", role.Name)
		selectors = append(selectors, selector)
	}

	selected := make([]string, 0)
	for name, other := range roles {
		if name == role.Name {
			continue
		}
		if slices.ContainsFunc(selectors, func(selector labels.Selector) bool {
			return selector.Matches(labels.Set(other.Labels))
		}) {
			selected = append(selected, name)
		}
	}

	return selected
}

// selectorModules returns the values of the module label a selector requires, from its matchLabels
// or from an In expression.
func selectorModules(selector metav1.LabelSelector) []string {
	if module, ok := selector.MatchLabels[moduleLabel]; ok {
		return []string{module}
	}

	for _, expression := range selector.MatchExpressions {
		if expression.Key == moduleLabel && expression.Operator == metav1.LabelSelectorOpIn {
			return expression.Values
		}
	}

	return nil
}
