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

package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

func TestRefusedManifestObjects(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	rule := func(namespace, name, level string) string {
		object := "---\napiVersion: deckhouse.io/v1alpha1\nkind: AuthorizationRule\nmetadata:\n  name: " + name + "\n"
		if namespace != "" {
			object += "  namespace: " + namespace + "\n"
		}
		object += "spec:\n  subjects:\n  - kind: User\n    name: alice\n"
		if level != "" {
			object += "  accessLevel: " + level + "\n"
		}
		return object
	}
	const configMap = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: info\n"
	const refusal = " of the Admin level, set the administrators of the project in .spec.administrators of the Project"

	tests := []struct {
		name     string
		rendered string
		want     []string
	}{
		{name: "nothing refused", rendered: configMap},
		{
			name:     "an Admin rule without a namespace is named in the main namespace",
			rendered: configMap + rule("", "admins", "Admin"),
			want:     []string{"AuthorizationRule proj/admins" + refusal},
		},
		{
			name:     "the other levels are not named",
			rendered: rule("", "users", "User") + rule("", "privileged", "PrivilegedUser") + rule("", "editors", "Editor"),
		},
		{
			name:     "a rule without an access level is not named",
			rendered: rule("", "nobody", ""),
		},
		{
			name:     "a rule with an access level that is not a string is not named",
			rendered: "---\napiVersion: deckhouse.io/v1alpha1\nkind: AuthorizationRule\nmetadata:\n  name: odd\nspec:\n  accessLevel: [Admin]\n",
		},
		{
			name:     "a rule without a spec is not named",
			rendered: "---\napiVersion: deckhouse.io/v1alpha1\nkind: AuthorizationRule\nmetadata:\n  name: bare\n",
		},
		{
			name: "every Admin rule in the order of the render",
			rendered: rule("proj-dev", "dev-admins", "Admin") + rule("proj-dev", "editors", "Editor") + configMap +
				rule("proj", "admins", "Admin"),
			want: []string{"AuthorizationRule proj-dev/dev-admins" + refusal, "AuthorizationRule proj/admins" + refusal},
		},
		{
			name: "the levels of rules inside a List",
			rendered: "---\napiVersion: v1\nkind: List\nitems:\n" +
				"- apiVersion: deckhouse.io/v1alpha1\n  kind: AuthorizationRule\n  metadata:\n    name: admins\n  spec:\n    accessLevel: Admin\n" +
				"- apiVersion: deckhouse.io/v1alpha1\n  kind: AuthorizationRule\n  metadata:\n    name: editors\n  spec:\n    accessLevel: Editor\n",
			want: []string{"AuthorizationRule proj/admins" + refusal},
		},
		{
			name:     "a kind of the same name in another group",
			rendered: "---\napiVersion: example.com/v1\nkind: AuthorizationRule\nmetadata:\n  name: admins\nspec:\n  accessLevel: Admin\n",
		},
		{
			name:     "a render that does not decode is named up to the failing document",
			rendered: rule("", "admins", "Admin") + "---\nkind: ConfigMap\nmetadata:\n  name: info\n",
			want:     []string{"AuthorizationRule proj/admins" + refusal},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, refusedNames(RefusedManifestObjects(project, tt.rendered)))
		})
	}

	// The status does not tell an object that exists from one that was refused: an object goes on
	// status.manifestObjects before its apply. The webhook reads the objects instead.
	t.Run("a rule the status lists is named all the same", func(t *testing.T) {
		applied := project.DeepCopy()
		applied.Status.ManifestObjects = []v1alpha3.ManifestObject{
			{APIVersion: "deckhouse.io/v1alpha1", Kind: "AuthorizationRule", Namespace: "proj", Name: "admins"},
		}
		assert.Equal(t, []string{"AuthorizationRule proj/admins" + refusal},
			refusedNames(RefusedManifestObjects(applied, rule("", "admins", "Admin"))))
	})
}

// The apply of an object that exists is an update, and the policy refuses one that raises a rule to
// the Admin level, not one of a rule that is Admin already.
func TestRefusedManifestObject_RefusesUpdateOf(t *testing.T) {
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}
	refused := RefusedManifestObjects(project,
		"---\napiVersion: deckhouse.io/v1alpha1\nkind: AuthorizationRule\nmetadata:\n  name: admins\nspec:\n  accessLevel: Admin\n")
	require.Len(t, refused, 1)

	bob := map[string]any{"kind": "User", "name": "bob"}
	eve := map[string]any{"kind": "User", "name": "eve"}
	refusedWithSubjects := func(subjects string) RefusedManifestObject {
		found := RefusedManifestObjects(project, "---\napiVersion: deckhouse.io/v1alpha1\nkind: AuthorizationRule\nmetadata:\n  name: admins\n"+
			"spec:\n  accessLevel: Admin\n  subjects:\n"+subjects)
		require.Len(t, found, 1)
		return found[0]
	}
	live := func(spec map[string]any) *unstructured.Unstructured {
		object := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "deckhouse.io/v1alpha1", "kind": "AuthorizationRule"}}
		if spec != nil {
			object.Object["spec"] = spec
		}
		return object
	}
	tests := []struct {
		name    string
		live    *unstructured.Unstructured
		refuses bool
	}{
		{name: "a rule of the Admin level stays Admin", live: live(map[string]any{"accessLevel": "Admin"})},
		{name: "an Editor rule is raised to Admin", live: live(map[string]any{"accessLevel": "Editor"}), refuses: true},
		{name: "a User rule is raised to Admin", live: live(map[string]any{"accessLevel": "User"}), refuses: true},
		{name: "a rule without an access level is raised to Admin", live: live(map[string]any{}), refuses: true},
		{name: "a rule without a spec is raised to Admin", live: live(nil), refuses: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.refuses, refused[0].RefusesUpdateOf(tt.live))
		})
	}

	// An Admin rule that stays Admin passes only while its subjects stay the same or fewer.
	adminLive := live(map[string]any{"accessLevel": "Admin", "subjects": []any{bob, eve}})
	assert.False(t, refusedWithSubjects("  - kind: User\n    name: bob\n").RefusesUpdateOf(adminLive), "a subject dropped")
	assert.False(t, refusedWithSubjects("  - kind: User\n    name: bob\n  - kind: User\n    name: eve\n").RefusesUpdateOf(adminLive), "the same subjects")
	assert.True(t, refusedWithSubjects("  - kind: User\n    name: mallory\n").RefusesUpdateOf(adminLive), "a subject added")
	assert.True(t, refusedWithSubjects("  - kind: Group\n    name: bob\n").RefusesUpdateOf(adminLive), "a subject of another kind")
}

func refusedNames(refused []RefusedManifestObject) []string {
	var names []string
	for _, object := range refused {
		names = append(names, object.String())
	}
	return names
}

// TestRefusedManifestKindsMatchThePolicy: the d8-multitenancy-manager-manifests admission policy
// refuses objects of the kinds, and the Project webhook warns of them from refusedManifestKinds. The
// two lists must be the same, or the webhook warns of what the API server allows or keeps silent
// about what it refuses. The policy must also select the users the controller applies the manifests
// as. The predicates are not compared with the expressions: their tests here and the tests of the
// policy in template_tests of the module go through the same cases.
func TestRefusedManifestKindsMatchThePolicy(t *testing.T) {
	policy := modulePolicy(t, "d8-multitenancy-manager-manifests")

	var matched []string
	for _, rule := range policy.Spec.MatchConstraints.ResourceRules {
		assert.Equal(t,
			[]admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			rule.Operations,
			"the policy matches creates and updates, and refusedManifestKind.refuses tells them apart by the live object")
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				matched = append(matched, group+"/"+resource)
			}
		}
	}
	var listed []string
	for _, kind := range refusedManifestKinds {
		listed = append(listed, kind.Group+"/"+kind.resource)
	}
	slices.Sort(matched)
	slices.Sort(listed)
	assert.Equal(t, listed, matched, "the resources the policy matches and the kinds the webhook warns of differ")

	// Every kind has a validation of its own, which names the kind by its resource.
	for _, kind := range listed {
		assert.True(t, slices.ContainsFunc(policy.Spec.Validations, func(validation admissionregistrationv1.Validation) bool {
			return strings.Contains(validation.Expression, fmt.Sprintf("%q", kind))
		}), "no validation of the policy names %s", kind)
	}

	require.Len(t, policy.Spec.MatchConditions, 1)
	prefix := regexp.MustCompile(`^request\.userInfo\.username\.startsWith\("([^"]*)"\)$`).FindStringSubmatch(policy.Spec.MatchConditions[0].Expression)
	require.NotNil(t, prefix, "the policy selects the users by a prefix")
	assert.Equal(t, naming.ProjectUser(""), prefix[1], "the policy selects other users than the controller applies the manifests as")
}

// modulePolicy reads a ValidatingAdmissionPolicy from templates/validation.yaml of the module. The
// template is not rendered: its Helm directives are dropped or filled in, which leaves plain YAML for
// the fields the tests read.
func modulePolicy(t *testing.T, name string) admissionregistrationv1.ValidatingAdmissionPolicy {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "..", "..", "templates", "validation.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "read the admission policy template at %s", path)

	directive := regexp.MustCompile(`\{\{[^}]*\}\}`)
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "{{ $policyName }}", "d8-multitenancy-manager"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "{{") {
			continue
		}
		lines = append(lines, directive.ReplaceAllString(line, "directive"))
	}
	for _, document := range strings.Split(strings.Join(lines, "\n"), "\n---\n") {
		var policy admissionregistrationv1.ValidatingAdmissionPolicy
		require.NoError(t, yaml.Unmarshal([]byte(document), &policy))
		if policy.Kind == "ValidatingAdmissionPolicy" && policy.Name == name {
			return policy
		}
	}
	t.Fatalf("no %s ValidatingAdmissionPolicy in %s", name, path)
	return admissionregistrationv1.ValidatingAdmissionPolicy{}
}
