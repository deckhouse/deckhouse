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
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

// TestManifestsFanOutMultiNamespace proves the namespaced objects (NetworkPolicy, PodLoggingConfig)
// are rendered once PER project namespace (main + additional from status.namespaces), while the
// cluster-scoped OperationPolicy and the main Namespace object are rendered once.
func TestManifestsFanOutMultiNamespace(t *testing.T) {
	t.Parallel()
	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			NetworkPolicy:    &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(v1alpha2.NetworkPolicyModeIsolated)},
			LogShipping:      &v1alpha2.LogShippingSpec{ClusterDestinationRef: v1alpha2.LiteralParam("central")},
			ParametersSchema: requiredRequestsSchema(map[string]any{"type": "boolean", "default": true}),
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

// TestManifestsRequiredRequests pins when the required-requests OperationPolicy is rendered: only when
// the template declares the requiredRequests parameter as a boolean and it resolves to true for the
// project.
func TestManifestsRequiredRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   v1alpha2.ParametersSchema
		params   map[string]any
		expected bool
	}{
		{
			name:     "template without a schema",
			expected: false,
		},
		{
			name: "schema without the parameter",
			schema: v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"team": map[string]any{"type": "string"}},
			}},
			expected: false,
		},
		{
			name:     "project sets a parameter the template does not declare",
			params:   map[string]any{"requiredRequests": true},
			expected: false,
		},
		{
			name:     "declared with default true",
			schema:   requiredRequestsSchema(map[string]any{"type": "boolean", "default": true}),
			expected: true,
		},
		{
			name:     "declared with default true and turned off by the project",
			schema:   requiredRequestsSchema(map[string]any{"type": "boolean", "default": true}),
			params:   map[string]any{"requiredRequests": false},
			expected: false,
		},
		{
			name:     "declared with default false",
			schema:   requiredRequestsSchema(map[string]any{"type": "boolean", "default": false}),
			expected: false,
		},
		{
			name:     "declared with default false and turned on by the project",
			schema:   requiredRequestsSchema(map[string]any{"type": "boolean", "default": false}),
			params:   map[string]any{"requiredRequests": true},
			expected: true,
		},
		{
			name:     "declared without a default",
			schema:   requiredRequestsSchema(map[string]any{"type": "boolean"}),
			expected: false,
		},
		{
			name:     "declared as a string",
			schema:   requiredRequestsSchema(map[string]any{"type": "string", "default": "true"}),
			expected: false,
		},
		{
			name: "declared with default true next to additionalProperties",
			schema: withAdditionalProperties(
				requiredRequestsSchema(map[string]any{"type": "boolean", "default": true}), true),
			expected: true,
		},
		{
			name: "declared with default false next to additionalProperties and turned on by the project",
			schema: withAdditionalProperties(
				requiredRequestsSchema(map[string]any{"type": "boolean", "default": false}), true),
			params:   map[string]any{"requiredRequests": true},
			expected: true,
		},
		{
			name: "declared with default true next to additionalProperties false",
			schema: withAdditionalProperties(
				requiredRequestsSchema(map[string]any{"type": "boolean", "default": true}), false),
			expected: true,
		},
		{
			name: "project sets an undeclared parameter that additionalProperties allows",
			schema: v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			}},
			params:   map[string]any{"requiredRequests": true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := &v1alpha2.ProjectTemplate{Spec: v1alpha2.ProjectTemplateSpec{ParametersSchema: tt.schema}}
			project := &v1alpha3.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "proj"},
				Spec:       v1alpha3.ProjectSpec{Parameters: tt.params},
			}

			out, err := Manifests(tmpl, project)
			require.NoError(t, err)
			require.Equal(t, tt.expected, strings.Contains(out, "kind: OperationPolicy"),
				"required-requests OperationPolicy rendered")
		})
	}
}

// requiredRequestsSchema is a parametersSchema declaring only the requiredRequests parameter.
func requiredRequestsSchema(property map[string]any) v1alpha2.ParametersSchema {
	return v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
		"type":       "object",
		"properties": map[string]any{"requiredRequests": property},
	}}
}

// withAdditionalProperties returns a copy of the schema with additionalProperties set on its root.
func withAdditionalProperties(schema v1alpha2.ParametersSchema, value any) v1alpha2.ParametersSchema {
	root := maps.Clone(schema.OpenAPIV3Schema)
	root["additionalProperties"] = value
	return v1alpha2.ParametersSchema{OpenAPIV3Schema: root}
}

// TestManifestsPodSecurityStandard pins how a podSecurityStandard parameter reaches the namespace:
// a profile name is rendered as the lower-case label value admission-policy-engine matches, and an
// empty value leaves the label out, even over a schema default, so the cluster default policy
// applies. Adoption of a namespace without the label relies on the empty value.
func TestManifestsPodSecurityStandard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   map[string]any
		expected string
		isSet    bool
	}{
		{name: "schema default", params: nil, expected: "baseline", isSet: true},
		{name: "restricted", params: map[string]any{"podSecurityProfile": v1alpha2.PodSecurityStandardRestricted}, expected: "restricted", isSet: true},
		{name: "privileged", params: map[string]any{"podSecurityProfile": v1alpha2.PodSecurityStandardPrivileged}, expected: "privileged", isSet: true},
		{name: "empty value wins over the default", params: map[string]any{"podSecurityProfile": ""}, isSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpl := &v1alpha2.ProjectTemplate{
				Spec: v1alpha2.ProjectTemplateSpec{
					PodSecurityStandard: v1alpha2.FromParamRef[string]("podSecurityProfile"),
					ParametersSchema: v1alpha2.ParametersSchema{
						OpenAPIV3Schema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"podSecurityProfile": map[string]any{"type": "string", "default": v1alpha2.PodSecurityStandardBaseline},
							},
						},
					},
				},
			}
			project := &v1alpha3.Project{
				ObjectMeta: metav1.ObjectMeta{Name: "proj"},
				Spec:       v1alpha3.ProjectSpec{Parameters: tt.params},
			}

			out, err := Manifests(tmpl, project)
			require.NoError(t, err)

			raw, isSet := namespaceLabels(t, out)["security.deckhouse.io/pod-policy"]
			value, _ := raw.(string)
			require.Equal(t, tt.isSet, isSet)
			require.Equal(t, tt.expected, value)
		})
	}
}

// namespaceLabels returns the labels of the Namespace in a multi-document render.
func namespaceLabels(t *testing.T, manifests string) map[string]any {
	t.Helper()

	for _, doc := range strings.Split(manifests, "---\n") {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var obj map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		if obj["kind"] != "Namespace" {
			continue
		}
		md, _ := obj["metadata"].(map[string]any)
		labels, _ := md["labels"].(map[string]any)

		return labels
	}

	t.Fatal("the render must contain the project namespace")

	return nil
}

// TestManifestsDedicatedPlacementBeatsMirroredAnnotation: adoption mirrors the placement annotations
// a namespace already had into the namespace parameter, so a template that declares nodeSelector or
// tolerations of its own must still win. Otherwise moving such a project onto a template with
// dedicated nodes reports Deployed while the pods keep landing where the stale mirror says.
func TestManifestsDedicatedPlacementBeatsMirroredAnnotation(t *testing.T) {
	t.Parallel()
	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			NodeSelector: v1alpha2.LiteralParam(map[string]string{"dedicated": "team"}),
			NamespaceMetadata: &v1alpha2.NamespaceMetadata{
				Annotations: v1alpha2.LiteralParam(map[string]string{
					naming.NodeSelectorAnnotation: "stale=mirror",
					"team":                        "blue",
				}),
			},
		},
	}
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "placed"}}

	out, err := Manifests(tmpl, project)
	require.NoError(t, err)

	found := false
	for _, doc := range strings.Split(out, "---\n") {
		var obj map[string]any
		if strings.TrimSpace(doc) == "" {
			continue
		}
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		if obj["kind"] != "Namespace" {
			continue
		}
		found = true
		md, _ := obj["metadata"].(map[string]any)
		ann, _ := md["annotations"].(map[string]any)
		require.Equal(t, "dedicated=team", ann[naming.NodeSelectorAnnotation],
			"spec.nodeSelector must outrank the mirrored annotation")
		require.Equal(t, "blue", ann["team"], "the rest of the mirrored annotations survive")
	}
	require.True(t, found, "the render must contain the project namespace")
}

// renderedNamespace returns the metadata of the project Namespace in a render.
func renderedNamespace(t *testing.T, out string) (labels, annotations map[string]any) {
	t.Helper()
	for _, doc := range strings.Split(out, "---\n") {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var obj map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
		if obj["kind"] != "Namespace" {
			continue
		}
		md, _ := obj["metadata"].(map[string]any)
		labels, _ = md["labels"].(map[string]any)
		annotations, _ = md["annotations"].(map[string]any)
		return labels, annotations
	}
	t.Fatal("the render must contain the project namespace")
	return nil, nil
}

// TestManifestsTemplateLabels: the labels a template puts on the main namespace through
// namespaceMetadata are listed, with their values, in the template-labels annotation, which the
// additional namespaces inherit from. Module-owned keys cannot be set through namespaceMetadata, so
// the dedicated fields win them; GitOps tracking labels stay on the main namespace but are not listed.
func TestManifestsTemplateLabels(t *testing.T) {
	t.Parallel()

	tmpl := &v1alpha2.ProjectTemplate{
		Spec: v1alpha2.ProjectTemplateSpec{
			PodSecurityStandard: v1alpha2.LiteralParam(v1alpha2.PodSecurityStandardBaseline),
			NamespaceMetadata: &v1alpha2.NamespaceMetadata{
				Labels: v1alpha2.LiteralParam(map[string]string{
					"team":                             "backend",
					"security.deckhouse.io/pod-policy": "privileged",
					"projects.deckhouse.io/project":    "other",
					"app.kubernetes.io/instance":       "argo-app",
					"argocd.argoproj.io/tracking-id":   "x",
				}),
			},
		},
	}
	project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}

	out, err := Manifests(tmpl, project)
	require.NoError(t, err)

	labels, annotations := renderedNamespace(t, out)
	require.Equal(t, "baseline", labels["security.deckhouse.io/pod-policy"], "the dedicated field wins a module key")
	require.NotContains(t, labels, "projects.deckhouse.io/project", "a module key cannot come from namespaceMetadata")
	require.Equal(t, "backend", labels["team"])
	require.Equal(t, "argo-app", labels["app.kubernetes.io/instance"], "a GitOps tracking label stays on the main namespace")
	require.JSONEq(t, `{"team":"backend"}`, annotations[naming.TemplateLabelsAnnotation].(string))
}

// TestManifestsTemplateLabelsAlwaysPresent: a template without namespaceMetadata still writes the
// annotation, empty, so an additional namespace can tell "no template labels" from "not rendered yet".
func TestManifestsTemplateLabelsAlwaysPresent(t *testing.T) {
	t.Parallel()

	out, err := Manifests(&v1alpha2.ProjectTemplate{}, &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj"}})
	require.NoError(t, err)

	_, annotations := renderedNamespace(t, out)
	require.Equal(t, "{}", annotations[naming.TemplateLabelsAnnotation])
}
