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

package render

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

// TestManifestsFanOutMultiNamespace proves the namespaced objects (NetworkPolicy, PodLoggingConfig)
// are rendered once PER project namespace (main + additional from status.namespaces), while the
// cluster-scoped OperationPolicy and the main Namespace object are rendered once.
func TestManifestsFanOutMultiNamespace(t *testing.T) {
	t.Parallel()
	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			NetworkPolicy: &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(v1alpha2.NetworkPolicyModeIsolated)},
			LogShipping:   &v1alpha2.LogShippingSpec{ClusterDestinationRef: v1alpha2.LiteralParam("central")},
		},
	}
	// Two additional namespaces plus a duplicate of the main entry to exercise sort + dedup.
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Status: v1alpha3.ProjectStatus{
			Namespaces: []v1alpha3.NamespaceStatus{
				{Name: "proj-b", Kind: v1alpha3.NamespaceKindAdditional},
				{Name: "proj", Kind: v1alpha3.NamespaceKindMain},
				{Name: "proj-a", Kind: v1alpha3.NamespaceKindAdditional},
				{Name: "proj", Kind: v1alpha3.NamespaceKindMain}, // duplicate main, must be deduped
			},
		},
	}

	out, err := Manifests(tmpl, project)
	require.NoError(t, err)

	byKind := map[string][]string{} // kind -> sorted namespaces of that kind
	for _, doc := range strings.Split(out, "---\n") {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var obj map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		kind, _ := obj["kind"].(string)
		ns := ""
		if md, ok := obj["metadata"].(map[string]any); ok {
			ns, _ = md["namespace"].(string)
		}
		byKind[kind] = append(byKind[kind], ns)
	}
	for k := range byKind {
		slices.Sort(byKind[k])
	}

	// Namespaced objects fan out into every project namespace, deduped and sorted.
	want := []string{"proj", "proj-a", "proj-b"}
	require.Equal(t, want, byKind["NetworkPolicy"], "NetworkPolicy must render into every project namespace")
	require.Equal(t, want, byKind["PodLoggingConfig"], "PodLoggingConfig must render into every project namespace")

	// Cluster-scoped OperationPolicy and the main Namespace are rendered once.
	require.Len(t, byKind["OperationPolicy"], 1, "OperationPolicy is cluster-scoped and rendered once")
	require.Len(t, byKind["Namespace"], 1, "only the main Namespace object is rendered (additional ns are owned by ProjectNamespace)")
}

// TestManifestsSingleNamespace keeps the main-only behaviour when the project has no additional
// namespaces (status not yet populated): exactly one NetworkPolicy in the main namespace.
func TestManifestsSingleNamespace(t *testing.T) {
	t.Parallel()
	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			NetworkPolicy: &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(v1alpha2.NetworkPolicyModeIsolated)},
		},
	}
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "solo"}}

	out, err := Manifests(tmpl, project)
	require.NoError(t, err)

	count := 0
	for _, doc := range strings.Split(out, "---\n") {
		var obj map[string]any
		if strings.TrimSpace(doc) == "" {
			continue
		}
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		if obj["kind"] == "NetworkPolicy" {
			count++
			md, _ := obj["metadata"].(map[string]any)
			require.Equal(t, "solo", md["namespace"])
		}
	}
	require.Equal(t, 1, count, "single-namespace project renders exactly one NetworkPolicy")
}
