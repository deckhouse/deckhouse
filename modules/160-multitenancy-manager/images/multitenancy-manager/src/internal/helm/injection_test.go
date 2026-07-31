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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

// clusterAdminPayload ends the value it is substituted into and continues as a new object. Applied,
// it would grant its subject cluster-admin, because the manifests are applied by a ServiceAccount
// that holds it.
const clusterAdminPayload = `evil
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: escalation
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: User
  name: attacker
`

func injectionTestClient(t *testing.T) *Client {
	t.Helper()

	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)

	return &Client{templates: templates, logger: ctrl.Log.WithName("test")}
}

// customTemplate is the shape a cluster administrator writes: one free-form string parameter and a
// resourcesTemplate that puts it somewhere.
func customTemplate(parameter, resources string) *v1alpha1.ProjectTemplate {
	template := new(v1alpha1.ProjectTemplate)
	template.Name = "custom"
	template.Spec.ParametersSchema.OpenAPIV3Schema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			parameter: map[string]any{"type": "string"},
		},
	}
	template.Spec.ResourcesTemplate = resources

	return template
}

func shippedTemplate(t *testing.T, name string) *v1alpha1.ProjectTemplate {
	t.Helper()

	template, err := read[v1alpha1.ProjectTemplate]("../../templates/" + name + ".yaml")
	require.NoError(t, err)

	return template
}

// The built-in templates render natively now, but a resourcesTemplate written by a cluster
// administrator still goes through the Helm engine, and its author is not obliged to know that an
// unquoted substitution ends at the first line break. The check needs to know nothing about the
// template: it compares what the parameters rendered into.
func TestEnsureParametersStayValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		template    func(t *testing.T) *v1alpha1.ProjectTemplate
		parameters  map[string]any
		refused     bool
		messagePart string
	}{
		{
			name: "an injected object",
			template: func(*testing.T) *v1alpha1.ProjectTemplate {
				return customTemplate("owner", `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: owner
data:
  owner: {{ .parameters.owner }}
`)
			},
			parameters: map[string]any{"owner": clusterAdminPayload},
			refused:    true,
		},
		{
			// The injection kept small enough that the manifests parse either way: the difference is
			// not a parse error but an object that came out unlike the one the parameters describe.
			name: "an injected list item",
			template: func(*testing.T) *v1alpha1.ProjectTemplate {
				return customTemplate("destination", `---
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: logs
spec:
  clusterDestinationRefs:
    - {{ .parameters.destination }}
`)
			},
			parameters:  map[string]any{"destination": "mine\n    - somebody-elses"},
			refused:     true,
			messagePart: "PodLoggingConfig deckhouse.io/v1alpha1/logs",
		},
		{
			// Refusing this would make the check unusable for templates that legitimately take a
			// multi-line parameter.
			name: "a multi-line value in a quoted field",
			template: func(*testing.T) *v1alpha1.ProjectTemplate {
				return customTemplate("script", `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  script: {{ .parameters.script | quote }}
  block: |
{{ indent 4 .parameters.script }}
`)
			},
			parameters: map[string]any{"script": "first line\nsecond line\n"},
		},
		{
			// The case that first tripped this check in a live cluster. Nothing about it is an
			// injection: it is an annotation, and it stays one. The two renders differed by a single
			// trailing space, because a block scalar comes back from YAML without its final line
			// break while the same value with the break already replaced comes back with a space in
			// its place.
			name:     "a multi-line annotation through the shipped simple template",
			template: func(t *testing.T) *v1alpha1.ProjectTemplate { return shippedTemplate(t, "simple") },
			parameters: map[string]any{
				"namespace": map[string]any{
					"labels": map[string]any{"team": "platform"},
					"annotations": map[string]any{
						"owner-note": "first line\nsecond line: with a colon\n---\nand a separator that must stay text\n",
					},
				},
			},
		},
		{
			// toYaml writes whatever it is given as a value, key or not.
			name:     "a payload used as an annotation key",
			template: func(t *testing.T) *v1alpha1.ProjectTemplate { return shippedTemplate(t, "simple") },
			parameters: map[string]any{
				"namespace": map[string]any{
					"annotations": map[string]any{clusterAdminPayload: "value"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := new(v1alpha3.Project)
			project.Name = "test"
			project.Spec.Parameters = tt.parameters

			err := injectionTestClient(t).ensureParametersStayValues(project, tt.template(t))

			if !tt.refused {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, ErrParameterInjection)
			if tt.messagePart != "" {
				assert.Contains(t, err.Error(), tt.messagePart)
			}
		})
	}
}

// The payload is present in the rendered manifests -- as the value it was put in, which is the
// point. What matters is that it produced no object of its own.
func TestPayloadRendersAsAValueAndNothingElse(t *testing.T) {
	t.Parallel()

	client := injectionTestClient(t)

	project := new(v1alpha3.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{
		"namespace": map[string]any{
			"labels": map[string]any{"owner": clusterAdminPayload},
		},
	}

	manifests, err := client.renderTemplate(project, shippedTemplate(t, "simple"))
	require.NoError(t, err)

	objects, err := canonicalObjects(manifests)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	for _, description := range objects {
		assert.Contains(t, description, "Namespace")
	}
}

// A project the check has nothing to say about must not be rendered at all. The template here cannot
// render, so anything but an immediate return would surface as an error.
func TestNothingToCheckIsNotRendered(t *testing.T) {
	t.Parallel()

	client := injectionTestClient(t)
	broken := customTemplate("owner", "{{ this is not a template")

	for _, parameters := range []map[string]any{nil, {}, {"owner": "user@example.com"}} {
		project := new(v1alpha3.Project)
		project.Name = "test"
		project.Spec.Parameters = parameters

		require.NoError(t, client.ensureParametersStayValues(project, broken))
	}
}

// The line breaks a parameter can carry, and what the rewrite makes of them. A clean value is left
// untouched, which is exactly what lets the check skip its second render.
func TestLineBreakRewrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		parameters map[string]any
		expected   map[string]any
	}{
		{
			name:       "an ordinary name is left alone",
			parameters: map[string]any{"owner": "user@example.com"},
			expected:   map[string]any{"owner": "user@example.com"},
		},
		{
			name:       "a break in a value",
			parameters: map[string]any{"name": "first\nsecond"},
			expected:   map[string]any{"name": "first second"},
		},
		{
			name:       "a break in a map key, which is a break like any other",
			parameters: map[string]any{"evil\nkey": "value"},
			expected:   map[string]any{"evil key": "value"},
		},
		{
			name:       "a break inside a nested list",
			parameters: map[string]any{"list": []any{"a\rb"}},
			expected:   map[string]any{"list": []any{"a b"}},
		},
		// YAML ends a scalar on these as well, so a rewrite that only knew about \n would miss them.
		{
			name:       "a line separator and a next line character",
			parameters: map[string]any{"one": "evil\u2028---", "two": "evil\u0085---"},
			expected:   map[string]any{"one": "evil ---", "two": "evil ---"},
		},
		{
			name:       "values that are not strings",
			parameters: map[string]any{"enabled": true, "count": int64(3)},
			expected:   map[string]any{"enabled": true, "count": int64(3)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, rewriteStringsIn(tt.parameters, lineBreakToSpace.Replace))
		})
	}
}

// Only whitespace is touched: the round trip through YAML is what differs between the two renders,
// and everything else has to stay comparable.
func TestCollapseSpace(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "a b c", collapseSpace("  a \n b\t\tc  "))
	assert.Equal(t, map[string]any{"k": "a b"}, rewriteStringsIn(map[string]any{"k": "a\nb"}, collapseSpace))
	assert.Equal(t,
		map[string]any{"list": []any{"a b", int64(1)}},
		rewriteStringsIn(map[string]any{"list": []any{"a  b", int64(1)}}, collapseSpace))
}
