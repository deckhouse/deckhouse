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
	"sort"

	"github.com/flant/kube-client/manifest/releaseutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// injectedClusterRoleBinding is a YAML document appended to a value taken from a
// ClusterAuthorizationRule. A line break returns the rendered output to column 0, so unless the
// value is emitted as a quoted scalar the remaining lines become an extra manifest of the release.
const injectedClusterRoleBinding = "\n" +
	"---\n" +
	"apiVersion: rbac.authorization.k8s.io/v1\n" +
	"kind: ClusterRoleBinding\n" +
	"metadata:\n" +
	"  name: injected-by-a-rule\n" +
	"roleRef:\n" +
	"  apiGroup: rbac.authorization.k8s.io\n" +
	"  kind: ClusterRole\n" +
	"  name: cluster-admin\n" +
	"subjects: []"

const injectedClusterRoleBindingName = "injected-by-a-rule"

const bindingsTemplate = "cluster-role-bindings.yaml"

// hostileRoleName carries a legitimate-looking role name followed by the injected document.
const hostileRoleName = "cluster-write-all" + injectedClusterRoleBinding

// hostileSubjectName is the same payload on a field that is rendered through toYaml rather than
// interpolated, so it must already come out as a quoted scalar.
const hostileSubjectName = "Efrem Testenev" + injectedClusterRoleBinding

const injectionClusterAuthRule = `---
- name: testenev
  spec:
    accessLevel: Admin
    subjects:
    - kind: User
      name: Efrem Testenev
    additionalRoles:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-write-all
`

var _ = Describe("Module :: user-authz :: helm template :: rule value injection", func() {
	f := SetupHelmConfig(``)

	var rendered map[string]string

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("global.enabledModules", `[]`)
		f.ValuesSet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", "test")

		f.ValuesSet("userAuthz.enableMultiTenancy", false)
		f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", true)
		f.ValuesSetFromYaml("userAuthz.internal.authRuleCrds", `[]`)
		f.ValuesSetFromYaml("userAuthz.internal.customClusterRoles", `{}`)
		f.ValuesSet("userAuthz.internal.webhookCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.key", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.key", "test")

		f.ValuesSetFromYaml("userAuthz.internal.clusterAuthRuleCrds", injectionClusterAuthRule)

		rendered = map[string]string{}
	})

	Context("With a ClusterAuthorizationRule carrying a YAML document in an additionalRoles name", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.internal.clusterAuthRuleCrds.0.spec.additionalRoles.0.name", hostileRoleName)
			f.HelmRender(WithFilteredRenderOutput(rendered, []string{bindingsTemplate}))
		})

		It("Should not add an object to the release", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", injectedClusterRoleBindingName).Exists()).
				To(BeFalse(), "the value must not become a manifest of its own")

			Expect(renderedBindings(rendered)).To(HaveLen(2),
				"%s must render exactly the additional-role and the accessLevel binding", bindingsTemplate)
		})

		It("Should keep the value a single scalar of the roleRef", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			binding := bindingForRole(rendered, hostileRoleName)
			Expect(binding).ToNot(BeNil(), "the additional-role binding must reference the value verbatim")
			Expect(binding.GetKind()).To(Equal("ClusterRoleBinding"))
			Expect(binding.GetName()).To(Equal("user-authz:testenev:additional-role:" + hostileRoleName))
		})

		It("Should keep the accessLevel binding intact", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			binding := bindingForRole(rendered, "user-authz:admin")
			Expect(binding).ToNot(BeNil())
			Expect(binding.GetName()).To(Equal("user-authz:testenev:admin"))

			subjects, _, err := unstructured.NestedSlice(binding.Object, "subjects")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(subjects).To(HaveLen(1))
			Expect(subjects[0].(map[string]interface{})["name"]).To(Equal("Efrem Testenev"))
		})
	})

	// subjects is serialised with toYaml, which quotes a value that needs it. This asserts that the
	// field is already safe rather than changing it.
	Context("With a ClusterAuthorizationRule carrying a YAML document in a subjects name", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.internal.clusterAuthRuleCrds.0.spec.subjects.0.name", hostileSubjectName)
			f.HelmRender(WithFilteredRenderOutput(rendered, []string{bindingsTemplate}))
		})

		It("Should not add an object to the release", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", injectedClusterRoleBindingName).Exists()).
				To(BeFalse(), "toYaml must keep the value a scalar")

			Expect(renderedBindings(rendered)).To(HaveLen(2))
		})

		It("Should keep the value a single scalar of every subject", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, binding := range renderedBindings(rendered) {
				subjects, _, err := unstructured.NestedSlice(binding.Object, "subjects")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(subjects).To(HaveLen(1))
				Expect(subjects[0].(map[string]interface{})["name"]).To(Equal(hostileSubjectName))
			}
		})
	})
})

// renderedBindings parses every manifest of the rendered binding template, splitting the stream the
// way the release does, so a value that stays a single scalar cannot be mistaken for a manifest of
// its own.
func renderedBindings(rendered map[string]string) []*unstructured.Unstructured {
	objects := make([]*unstructured.Unstructured, 0)

	for _, manifests := range rendered {
		split := releaseutil.SplitManifests(manifests)

		names := make([]string, 0, len(split))
		for name := range split {
			names = append(names, name)
		}
		sort.Sort(releaseutil.BySplitManifestsOrder(names))

		for _, name := range names {
			object := ManifestStringToUnstructed(split[name])
			if object == nil {
				continue
			}

			objects = append(objects, object)
		}
	}

	return objects
}

// bindingForRole returns the rendered binding whose roleRef names the given role, or nil.
func bindingForRole(rendered map[string]string, role string) *unstructured.Unstructured {
	for _, object := range renderedBindings(rendered) {
		name, found, err := unstructured.NestedString(object.Object, "roleRef", "name")
		if err != nil || !found {
			continue
		}

		if name == role {
			return object
		}
	}

	return nil
}
