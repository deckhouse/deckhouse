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

package helm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	renderpkg "controller/internal/render"
)

// TestNativeRender proves the built-in templates, rendered natively from their structured fields
// (controller/internal/render) and run through the release post-renderer, produce the golden objects
// under testdata -- which are the objects the Helm-string versions of these templates once produced.
//
// String leaves are whitespace-normalized before comparison: the helm folded-scalar render of the
// Falco rule condition carries template-induced blank lines that a native render does not (and should
// not) reproduce; the content is otherwise identical.
func TestNativeRender(t *testing.T) {
	cases := []struct {
		tmplFile string
		caseDir  string
	}{
		{"simple.yaml", "simple_case"},
		{"default.yaml", "default_case"},
		{"secure.yaml", "secure_case"},
		{"secure-with-dedicated-nodes.yaml", "secure_with_dedicated_node_case"},
	}

	for _, c := range cases {
		t.Run(c.caseDir, func(t *testing.T) {
			tmpl, err := read[v1alpha2.ProjectTemplate](filepath.Join("../../templates", c.tmplFile))
			require.NoError(t, err)
			require.Empty(t, tmpl.Spec.ResourcesTemplate, "built-in templates must be structured, not helm strings")

			base := filepath.Join("./testdata", c.caseDir)
			project, err := read[v1alpha3.Project](filepath.Join(base, "project.yaml"))
			require.NoError(t, err)

			manifests, err := renderpkg.Manifests(tmpl, project)
			require.NoError(t, err)

			post := newPostRenderer(project, nil, ctrl.Log.WithName("test"), true)
			post.manifests = manifests
			out, err := post.Run(bytes.NewBuffer(nil))
			require.NoError(t, err)

			rawExpected, err := os.ReadFile(filepath.Join(base, "resources.yaml"))
			require.NoError(t, err)

			renderedMap := indexManifests(t, out.String())
			expectedMap := indexManifests(t, string(rawExpected))

			require.ElementsMatch(t, keys(expectedMap), keys(renderedMap), "rendered object set must match the golden set")
			for name, expected := range expectedMap {
				if diff := cmp.Diff(normalizeStrings(renderedMap[name].Object), normalizeStrings(expected.Object), cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("object %s does not match golden:\n%s", name, diff)
				}
			}
		})
	}
}

// TestNativeRenderSelectsWhatTheHelmTemplateSelected proves a project on a built-in template keeps
// what its Helm template release gave it when the structured built-in template renders it from the
// parameters the conversion made of the old ones. testdata/helm_template holds, per template, the
// project as the Helm template path saw it (project.yaml), the objects that path rendered from it
// (resources.yaml) and the converted project (converted.yaml, pinned to the shipped conversion by a
// test of the module hooks).
//
// The two renders are not equal byte for byte and are not meant to be: the NetworkPolicy and the
// OperationPolicy of the Helm template picked the project namespace by kubernetes.io/metadata.name,
// the structured ones pick every namespace of the project by projects.deckhouse.io/project. So every
// namespaceSelector is replaced by the namespaces it selects in a cluster where, as before, only the
// project namespace carries that label, and the objects are compared after that. The
// AuthorizationRule objects and the all-pods ResourceQuota are not rendered any more: the
// administrators and the quota are standard fields, applied as the d8-administrators
// ProjectRoleBinding (role d8:project:admin instead of the user-authz Admin level) and the
// d8-project-quota ResourceQuota, and they have to carry the same subjects and the same limits.
func TestNativeRenderSelectsWhatTheHelmTemplateSelected(t *testing.T) {
	for _, template := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(template, func(t *testing.T) {
			base := filepath.Join("./testdata", "helm_template", template)
			tmpl, err := read[v1alpha2.ProjectTemplate](filepath.Join("../../templates", template+".yaml"))
			require.NoError(t, err)
			project, err := read[v1alpha3.Project](filepath.Join(base, "converted.yaml"))
			require.NoError(t, err)

			manifests, err := renderpkg.Manifests(tmpl, project)
			require.NoError(t, err)
			post := newPostRenderer(project, nil, ctrl.Log.WithName("test"), true)
			post.manifests = manifests
			out, err := post.Run(bytes.NewBuffer(nil))
			require.NoError(t, err)
			native := indexManifests(t, out.String())

			rawHelm, err := os.ReadFile(filepath.Join(base, "resources.yaml"))
			require.NoError(t, err)
			helmRender := indexManifests(t, string(rawHelm))

			// the objects the standard fields replace
			var subjects []v1alpha3.Administrator
			for name, object := range helmRender {
				if object.GetKind() != "AuthorizationRule" {
					continue
				}
				level, _, _ := unstructured.NestedString(object.Object, "spec", "accessLevel")
				assert.Equal(t, "Admin", level, name)
				ruleSubjects, _, _ := unstructured.NestedSlice(object.Object, "spec", "subjects")
				for _, s := range ruleSubjects {
					subject, ok := s.(map[string]any)
					require.True(t, ok)
					subjects = append(subjects, v1alpha3.Administrator{Kind: fmt.Sprint(subject["kind"]), Name: fmt.Sprint(subject["name"])})
				}
				delete(helmRender, name)
			}
			assert.ElementsMatch(t, project.Spec.Administrators, subjects, "the administrators are the subjects of the rules")

			quota, ok := helmRender["ResourceQuota.all-pods"]
			require.True(t, ok, "the Helm template rendered the all-pods quota")
			hard, _, _ := unstructured.NestedMap(quota.Object, "spec", "hard")
			require.Len(t, project.Spec.Quota, len(hard))
			for key, value := range hard {
				limit, err := resource.ParseQuantity(fmt.Sprint(value))
				require.NoError(t, err)
				converted, found := project.Spec.Quota[corev1.ResourceName(key)]
				require.True(t, found, "the quota keeps %s", key)
				assert.Zero(t, limit.Cmp(converted), "the quota keeps the limit of %s", key)
			}
			delete(helmRender, "ResourceQuota.all-pods")

			require.ElementsMatch(t, keys(helmRender), keys(native), "the structured render keeps every other object, under the same name")
			helmNamespaces := clusterNamespaces(helmRender["Namespace.test"])
			nativeNamespaces := clusterNamespaces(native["Namespace.test"])
			for name, expected := range helmRender {
				if expected.GetKind() == "Namespace" {
					// Labels and the scheduling annotations are what a namespace does; an annotation the
					// Helm render did not set is bookkeeping of the controller.
					native[name].SetAnnotations(onlyKeys(native[name].GetAnnotations(), expected.GetAnnotations()))
				}
				got := selectedNamespaces(t, normalizeStrings(native[name].Object), nativeNamespaces)
				want := selectedNamespaces(t, normalizeStrings(expected.Object), helmNamespaces)
				if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("%s does not select what the Helm template selected:\n%s", name, diff)
				}
			}
		})
	}
}

// onlyKeys keeps the entries of have whose keys want has; nil when none is left, so the object
// carries no annotations at all rather than an empty map.
func onlyKeys(have, want map[string]string) map[string]string {
	var out map[string]string
	for key := range want {
		if value, ok := have[key]; ok {
			if out == nil {
				out = make(map[string]string, len(want))
			}
			out[key] = value
		}
	}
	return out
}

// clusterNamespaces is the cluster the selectors are evaluated in: the project namespace with the
// labels the render gave it, the platform namespaces the policies name, and a namespace of another
// project. Kubernetes labels every namespace with its own name. Only the project namespace carries
// projects.deckhouse.io/project=test, which is where the two ways of selecting it agree.
func clusterNamespaces(project *unstructured.Unstructured) map[string]labels.Set {
	namespaces := map[string]labels.Set{
		"d8-monitoring":                {},
		"d8-ingress-nginx":             {},
		"d8-service-with-healthchecks": {},
		"kube-system":                  {},
		"default":                      {},
		"other": {
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
			v1alpha3.ResourceLabelProject:  "other",
		},
	}
	if project != nil {
		namespaces[project.GetName()] = labels.Set(project.GetLabels())
	}
	for name, set := range namespaces {
		withName := labels.Merge(set, labels.Set{corev1.LabelMetadataName: name})
		namespaces[name] = withName
	}
	return namespaces
}

// selectedNamespaces replaces every namespaceSelector in the object -- a LabelSelector, or one
// wrapped in labelSelector as the admission-policy-engine policies spell it -- with the sorted names
// of the namespaces it selects.
func selectedNamespaces(t *testing.T, node any, namespaces map[string]labels.Set) any {
	t.Helper()
	switch value := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			if key != "namespaceSelector" {
				out[key] = selectedNamespaces(t, child, namespaces)
				continue
			}
			raw, ok := child.(map[string]any)
			require.True(t, ok, "a namespaceSelector is an object")
			if wrapped, ok := raw["labelSelector"].(map[string]any); ok {
				raw = wrapped
			}
			selector := new(metav1.LabelSelector)
			require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(raw, selector))
			compiled, err := metav1.LabelSelectorAsSelector(selector)
			require.NoError(t, err)
			var selected []any
			for name, set := range namespaces {
				if compiled.Matches(set) {
					selected = append(selected, name)
				}
			}
			slices.SortFunc(selected, func(a, b any) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
			out[key] = map[string]any{"selects": selected}
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = selectedNamespaces(t, child, namespaces)
		}
		return out
	default:
		return node
	}
}

func indexManifests(t *testing.T, raw string) map[string]*unstructured.Unstructured {
	t.Helper()
	out := make(map[string]*unstructured.Unstructured)
	for _, doc := range releaseutil.SplitManifests(raw) {
		object, ok, err := parseManifest(doc)
		require.NoError(t, err)
		if !ok {
			continue
		}
		out[fmt.Sprintf("%s.%s", object.GetKind(), object.GetName())] = object
	}
	return out
}

func keys(m map[string]*unstructured.Unstructured) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// normalizeStrings collapses internal whitespace runs in every string leaf so the comparison ignores
// the helm folded-scalar artifacts while still catching real content differences.
func normalizeStrings(v any) any {
	switch o := v.(type) {
	case map[string]any:
		for k, val := range o {
			o[k] = normalizeStrings(val)
		}
		return o
	case []any:
		for i, val := range o {
			o[i] = normalizeStrings(val)
		}
		return o
	case string:
		return strings.Join(strings.Fields(o), " ")
	default:
		return v
	}
}
