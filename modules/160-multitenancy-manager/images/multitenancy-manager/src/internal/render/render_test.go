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

// TestFalcoRuleCoversEveryProjectNamespace guards the one cluster-scoped object that cannot use a
// label selector: a Falco condition names namespaces explicitly, so the drift rule has to enumerate
// them or it would watch only the main namespace of a multi-namespace project.
func TestFalcoRuleCoversEveryProjectNamespace(t *testing.T) {
	t.Parallel()
	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			AllowedUIDs:  v1alpha2.LiteralParam(v1alpha2.IDRange{Min: 1000, Max: 2000}),
			RuntimeAudit: &v1alpha2.RuntimeAuditSpec{Enabled: v1alpha2.LiteralParam(true)},
		},
	}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Status: v1alpha3.ProjectStatus{
			Namespaces: []v1alpha3.NamespaceStatus{
				{Name: "proj", Kind: v1alpha3.NamespaceKindMain},
				{Name: "proj-extra", Kind: v1alpha3.NamespaceKindAdditional},
			},
		},
	}

	out, err := Manifests(tmpl, project)
	require.NoError(t, err)

	condition := falcoCondition(t, out)
	require.Contains(t, condition, "k8s.ns.name in (proj, proj-extra)")
	require.NotContains(t, condition, "k8s.ns.name=proj")
}

// TestFalcoRuleSingleNamespaceKeepsEqualityForm pins the wording for the common case: the legacy
// resourcesTemplate renders plain equality, and the native renderer must not diverge from it.
func TestFalcoRuleSingleNamespaceKeepsEqualityForm(t *testing.T) {
	t.Parallel()
	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			AllowedUIDs:  v1alpha2.LiteralParam(v1alpha2.IDRange{Min: 1000, Max: 2000}),
			RuntimeAudit: &v1alpha2.RuntimeAuditSpec{Enabled: v1alpha2.LiteralParam(true)},
		},
	}
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "solo"}}

	out, err := Manifests(tmpl, project)
	require.NoError(t, err)

	require.Contains(t, falcoCondition(t, out), "k8s.ns.name=solo")
}

// falcoCondition extracts the condition of the rendered container-drift rule.
func falcoCondition(t *testing.T, manifests string) string {
	t.Helper()

	for _, doc := range strings.Split(manifests, "---\n") {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var obj map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		if kind, _ := obj["kind"].(string); kind != "FalcoAuditRules" {
			continue
		}
		spec, _ := obj["spec"].(map[string]any)
		rules, _ := spec["rules"].([]any)
		for _, raw := range rules {
			entry, _ := raw.(map[string]any)
			rule, ok := entry["rule"].(map[string]any)
			if !ok {
				continue
			}
			condition, _ := rule["condition"].(string)
			return condition
		}
	}

	t.Fatal("no FalcoAuditRules rule rendered")

	return ""
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
