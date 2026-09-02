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
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/internal/validate"
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
			// The other direction: the line break costs the project an object instead of gaining it
			// one. Suppressing the NetworkPolicy that isolates a project is worth as much to an
			// attacker as adding a ClusterRoleBinding, so the comparison has to look both ways.
			name: "a suppressed object",
			template: func(*testing.T) *v1alpha1.ProjectTemplate {
				return customTemplate("tier", `
{{- if eq .parameters.tier "gold plated" }}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: isolated
spec:
  podSelector: {}
{{- end }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: tier
data:
  tier: {{ .parameters.tier | quote }}
`)
			},
			parameters:  map[string]any{"tier": "gold\nplated"},
			refused:     true,
			messagePart: "removing: NetworkPolicy networking.k8s.io/v1/isolated",
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
			// The shipped template quotes the substitution, so the payload stays the administrator's
			// name. The schema refuses it before this point, but the template has to answer for
			// itself: a copy of it in somebody's cluster carries no schema of ours.
			name: "a payload as an administrator name in the shipped default template",
			template: func(t *testing.T) *v1alpha1.ProjectTemplate {
				template := shippedTemplate(t, "default")
				stripAdministratorPattern(template)

				return template
			},
			parameters: map[string]any{
				"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}},
				"administrators": []any{map[string]any{"subject": "User", "name": clusterAdminPayload}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := new(v1alpha2.Project)
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

// The payload is present in the rendered manifests -- as the administrator's name, which is where it
// was put. What matters is that it produced no object of its own.
func TestPayloadRendersAsAValueAndNothingElse(t *testing.T) {
	t.Parallel()

	client := injectionTestClient(t)
	template := shippedTemplate(t, "default")
	stripAdministratorPattern(template)

	project := new(v1alpha2.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{
		"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}},
		"administrators": []any{map[string]any{"subject": "User", "name": clusterAdminPayload}},
	}

	manifests, err := client.renderTemplate(project, template)
	require.NoError(t, err)

	objects, err := canonicalObjects(manifests)
	require.NoError(t, err)
	for _, description := range objects {
		assert.NotContains(t, description, "ClusterRoleBinding")
	}
}

// The schema is the barrier in front of the template: a line break has no place in a subject name,
// and the value is refused before anything is rendered.
func TestAdministratorNameWithLineBreakIsRefusedBySchema(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			template := shippedTemplate(t, name)
			require.NoError(t, validate.ProjectTemplate(template))

			project := new(v1alpha2.Project)
			project.Name = "test"
			project.Spec.ProjectTemplateName = name
			project.Spec.Parameters = map[string]any{
				"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}},
				"administrators": []any{map[string]any{"subject": "User", "name": clusterAdminPayload}},
			}

			err := validate.Project(project, template)
			require.Error(t, err, "the schema accepted a subject name carrying a line break")
			assert.Contains(t, err.Error(), "administrators")
		})
	}
}

// Pinning the log destination to a Kubernetes object name must not take the empty string with it:
// the template skips the whole logging block for it, so that is how a project turns logging off, and
// the schema is checked on every reconcile rather than only on edit -- refusing it would have broken
// such a project without anybody touching it.
func TestEmptyLogDestinationStaysAllowed(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			template := shippedTemplate(t, name)
			require.NoError(t, validate.ProjectTemplate(template))

			project := new(v1alpha2.Project)
			project.Name = "test"
			project.Spec.ProjectTemplateName = name
			project.Spec.Parameters = map[string]any{
				"resourceQuota":             map[string]any{"requests": map[string]any{"cpu": "1"}},
				"administrators":            []any{map[string]any{"subject": "User", "name": "user@example.com"}},
				"clusterLogDestinationName": "",
			}

			require.NoError(t, validate.Project(project, template))

			manifests, err := injectionTestClient(t).renderTemplate(project, template)
			require.NoError(t, err)
			assert.NotContains(t, manifests, "PodLoggingConfig", "an empty destination still rendered the logging block")

			// And the pattern still does its work on a value that is not empty.
			project.Spec.Parameters["clusterLogDestinationName"] = "loki\n---"
			assert.Error(t, validate.Project(project, template))
		})
	}
}

// Quoting the administrator name keeps a digit-only value (for example "008") a YAML
// string. Without quote, the YAML parser turns it into a number and Kubernetes rejects
// AuthorizationRule.metadata.name / subjects[].name as the wrong type.
func TestDigitOnlyAdministratorNameStaysAString(t *testing.T) {
	t.Parallel()

	client := injectionTestClient(t)
	const digitOnlyName = "008"

	for _, templateName := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(templateName, func(t *testing.T) {
			t.Parallel()

			project := new(v1alpha2.Project)
			project.Name = "test"
			project.Spec.Parameters = map[string]any{
				"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}},
				"administrators": []any{map[string]any{"subject": "User", "name": digitOnlyName}},
			}

			manifests, err := client.renderTemplate(project, shippedTemplate(t, templateName))
			require.NoError(t, err)

			rule := findObject(t, manifests, "AuthorizationRule")
			assert.Equal(t, digitOnlyName, rule.GetName(), "metadata.name must stay a string after YAML round-trip")

			subjects, found, err := unstructured.NestedSlice(rule.Object, "spec", "subjects")
			require.NoError(t, err)
			require.True(t, found)
			require.Len(t, subjects, 1)
			subject, ok := subjects[0].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, digitOnlyName, subject["name"])
		})
	}
}

// Quoting the quota substitutions turned the rendered values into strings. That is the canonical
// form of a Quantity, but the golden fixtures render a copy of the template rather than the shipped
// one, so nothing else would notice if it stopped parsing.
func TestQuotedQuotaStaysAQuantity(t *testing.T) {
	t.Parallel()

	client := injectionTestClient(t)

	project := new(v1alpha2.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{
		"resourceQuota": map[string]any{
			// A number, the way the documented example writes it.
			"requests": map[string]any{"cpu": 1, "memory": "1Gi"},
			"limits":   map[string]any{"memory": "15Gi"},
		},
		"administrators": []any{map[string]any{"subject": "User", "name": "user@example.com"}},
	}

	manifests, err := client.renderTemplate(project, shippedTemplate(t, "default"))
	require.NoError(t, err)

	quota := findObject(t, manifests, "ResourceQuota")
	hard, found, err := unstructured.NestedStringMap(quota.Object, "spec", "hard")
	require.NoError(t, err)
	require.True(t, found, "the quota is not a map of strings")

	for name, value := range hard {
		_, err = resource.ParseQuantity(value)
		assert.NoError(t, err, "%s = %q is not a quantity", name, value)
	}
	assert.Equal(t, "1", hard["requests.cpu"])
}

func findObject(t *testing.T, manifests, kind string) *unstructured.Unstructured {
	t.Helper()

	for _, raw := range releaseutil.SplitManifests(manifests) {
		object := new(unstructured.Unstructured)
		require.NoError(t, yaml.Unmarshal([]byte(raw), object))
		if object.GetKind() == kind {
			return object
		}
	}

	t.Fatalf("no %s in the rendered manifests", kind)

	return nil
}

// stripAdministratorPattern removes the schema constraint on administrator names, leaving the
// template to answer for itself.
func stripAdministratorPattern(template *v1alpha1.ProjectTemplate) {
	properties, _ := template.Spec.ParametersSchema.OpenAPIV3Schema["properties"].(map[string]interface{})
	administrators, _ := properties["administrators"].(map[string]interface{})
	items, _ := administrators["items"].(map[string]interface{})
	itemProperties, _ := items["properties"].(map[string]interface{})
	name, _ := itemProperties["name"].(map[string]interface{})
	delete(name, "pattern")
}

// A project the check has nothing to say about must not be rendered at all. The template here cannot
// render, so anything but an immediate return would surface as an error.
func TestNothingToCheckIsNotRendered(t *testing.T) {
	t.Parallel()

	client := injectionTestClient(t)
	broken := customTemplate("owner", "{{ this is not a template")

	for _, parameters := range []map[string]any{nil, {}, {"owner": "user@example.com"}} {
		project := new(v1alpha2.Project)
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
