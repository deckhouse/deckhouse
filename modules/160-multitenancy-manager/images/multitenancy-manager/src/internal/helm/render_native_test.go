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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	renderpkg "controller/internal/render"
)

// TestNativeRender proves the schema-based (v1alpha2) built-in templates, rendered natively from
// their structured fields (controller/internal/render) and run through the same post-renderer as the
// legacy helm path, produce the objects the legacy resourcesTemplate path produced — the golden
// testdata resources.yaml. This is the no-regression guarantee for the ADR-3 rewrite.
//
// String leaves are whitespace-normalized before comparison: the helm folded-scalar render of the
// Falco rule condition carries template-induced blank lines that a native render does not (and should
// not) reproduce; the content is otherwise identical.
func TestNativeRender(t *testing.T) {
	cases := []struct {
		tmplFile string
		caseDir  string
	}{
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
			out, err := post.Run(bytes.NewBufferString(manifests))
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
