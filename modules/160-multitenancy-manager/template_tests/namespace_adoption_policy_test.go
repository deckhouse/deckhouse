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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const namespaceAdoptionPolicy = "d8-multitenancy-manager-namespace-adoption"

// namespaceObjectWrite is a create of the named namespace with the labels, or an update of it from the
// old labels when they are not nil. The writer is in the groups its name implies and in the extra ones.
func namespaceObjectWrite(name, username string, oldLabels, labels map[string]string, extraGroups ...string) admission.Attributes {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	namespace := func(labels map[string]string) *unstructured.Unstructured {
		object := new(unstructured.Unstructured)
		object.SetGroupVersionKind(gvk)
		object.SetName(name)
		object.SetLabels(labels)
		return object
	}
	operation := admission.Create
	var oldObject runtime.Object
	if oldLabels != nil {
		operation = admission.Update
		oldObject = namespace(oldLabels)
	}
	return admission.NewAttributesRecord(namespace(labels), oldObject, gvk, "", name, schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
		"", operation, nil, false, &user.DefaultInfo{Name: username, Groups: append(groupsOf(username), extraGroups...)})
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: namespace adoption policy ::", func() {
	hec := SetupHelmConfig("")

	for _, allow := range []bool{true, false} {
		It(fmt.Sprintf("keeps every namespace adoptable with allowNamespacesWithoutProjects: %v", allow), func() {
			hec.ValuesSet("multitenancyManager.allowNamespacesWithoutProjects", allow)
			renderQuota(hec, true)
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", namespaceAdoptionPolicy).Exists()).To(BeTrue())
			policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", namespaceAdoptionPolicy).ToYaml())

			const user = "argocd@example.com"
			upmeter := map[string]string{"heritage": "upmeter"}
			type adoptionCase struct {
				name    string
				attrs   admission.Attributes
				allowed bool
			}
			cases := []adoptionCase{
				{name: "an ordinary namespace", attrs: namespaceObjectWrite("app", user, nil, nil), allowed: true},
				{name: "a name of 61 characters", attrs: namespaceObjectWrite(strings.Repeat("a", 61), user, nil, nil), allowed: true},
				{name: "a name of 62 characters", attrs: namespaceObjectWrite(strings.Repeat("a", 62), user, nil, nil)},
				{name: "the upmeter- prefix", attrs: namespaceObjectWrite("upmeter-x", user, nil, nil)},
				{name: "the name of the virtual project", attrs: namespaceObjectWrite("deckhouse", user, nil, nil)},
				{name: "a name that only starts like the virtual project", attrs: namespaceObjectWrite("deckhouse-x", user, nil, nil), allowed: true},
				{name: "an update of the namespace named like the virtual project", attrs: namespaceObjectWrite("deckhouse", user, map[string]string{}, map[string]string{"x": "y"}), allowed: true},
				{name: "heritage: upmeter on create", attrs: namespaceObjectWrite("app", user, nil, upmeter)},
				{name: "heritage: upmeter put on later", attrs: namespaceObjectWrite("app", user, map[string]string{}, upmeter)},
				{name: "a namespace that had it keeps it", attrs: namespaceObjectWrite("app", user, upmeter, map[string]string{"heritage": "upmeter", "x": "y"}), allowed: true},
				{name: "the upmeter agent", attrs: namespaceObjectWrite("upmeter-probe", "system:serviceaccount:d8-upmeter:upmeter-agent", nil, upmeter), allowed: true},
				{name: "a cluster administrator", attrs: namespaceObjectWrite(strings.Repeat("a", 63), "system:sudouser", nil, nil), allowed: true},
			}
			// The writers the policy leaves out, by the identities of its matchCondition: each of them may
			// create a namespace that is too long, one with the upmeter- prefix and the one named like
			// the virtual project.
			excluded := []struct {
				name     string
				username string
				groups   []string
			}{
				{name: "the API server", username: "system:apiserver"},
				{name: "a member of system:masters", username: "admin@example.com", groups: []string{"system:masters"}},
				{name: "a node", username: "system:node:worker-0", groups: []string{"system:nodes"}},
				{name: "Deckhouse", username: "system:serviceaccount:d8-system:deckhouse"},
				{name: "a service account of d8-system", username: "system:serviceaccount:d8-system:other"},
				{name: "the module controller", username: "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"},
				{name: "a service account of kube-system", username: "system:serviceaccount:kube-system:generic-garbage-collector"},
			}
			for _, writer := range excluded {
				for _, name := range []string{strings.Repeat("a", 62), "upmeter-x", "deckhouse"} {
					cases = append(cases, adoptionCase{
						name:    fmt.Sprintf("%s creates %q", writer.name, name),
						attrs:   namespaceObjectWrite(name, writer.username, nil, nil, writer.groups...),
						allowed: true,
					})
				}
			}
			for _, tc := range cases {
				allowed, _, err := policy.admit(tc.attrs, nil)
				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
			}
		})
	}
})
