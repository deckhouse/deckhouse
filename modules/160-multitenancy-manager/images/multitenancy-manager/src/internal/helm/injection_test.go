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
	"strings"
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

// stringParameterSchema declares one free-form string parameter, the shape a template author writes
// when the value is just text.
func stringParameterSchema(name string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			name: map[string]any{"type": "string"},
		},
	}
}

// The built-in templates render natively now, but a resourcesTemplate written by a cluster
// administrator still goes through the Helm engine, and its author is not obliged to know that an
// unquoted substitution ends at the first line break. The check needs to know nothing about the
// template: it compares what the parameters rendered into.
func TestInjectedObjectIsRefusedForAnUnquotedTemplate(t *testing.T) {
	client := injectionTestClient(t)

	template := new(v1alpha1.ProjectTemplate)
	template.Name = "unquoted"
	template.Spec.ParametersSchema.OpenAPIV3Schema = stringParameterSchema("owner")
	template.Spec.ResourcesTemplate = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: owner
data:
  owner: {{ .parameters.owner }}
`

	project := new(v1alpha3.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{"owner": clusterAdminPayload}

	require.ErrorIs(t, client.ensureParametersStayValues(project, template), ErrParameterInjection)
}

// The same check with the injection kept small enough that the manifests parse either way: here the
// difference is not a parse error but an object that came out unlike the one the parameters describe.
func TestInjectedListItemIsRefusedAndNamed(t *testing.T) {
	client := injectionTestClient(t)

	template := new(v1alpha1.ProjectTemplate)
	template.Name = "unquoted-list"
	template.Spec.ParametersSchema.OpenAPIV3Schema = stringParameterSchema("destination")
	template.Spec.ResourcesTemplate = `---
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: logs
spec:
  clusterDestinationRefs:
    - {{ .parameters.destination }}
`

	project := new(v1alpha3.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{"destination": "mine\n    - somebody-elses"}

	err := client.ensureParametersStayValues(project, template)
	require.ErrorIs(t, err, ErrParameterInjection)
	assert.Contains(t, err.Error(), "PodLoggingConfig deckhouse.io/v1alpha1/logs")
}

// A line break that stays inside a value is not injection, and refusing it would make the check
// unusable for templates that legitimately take a multi-line parameter.
func TestMultilineValueInAQuotedFieldIsAllowed(t *testing.T) {
	client := injectionTestClient(t)

	template := new(v1alpha1.ProjectTemplate)
	template.Name = "quoted"
	template.Spec.ParametersSchema.OpenAPIV3Schema = stringParameterSchema("script")
	template.Spec.ResourcesTemplate = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  script: {{ .parameters.script | quote }}
  block: |
{{ indent 4 .parameters.script }}
`

	project := new(v1alpha3.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{"script": "first line\nsecond line\n"}

	require.NoError(t, client.ensureParametersStayValues(project, template))
}

// The shipped simple template passes namespace labels and annotations through toYaml, which writes
// them as values whatever they contain. This is the assertion that keeps it that way.
func TestSimpleTemplateKeepsNamespaceMetadataAsValues(t *testing.T) {
	client := injectionTestClient(t)

	template, err := read[v1alpha1.ProjectTemplate]("../../templates/simple.yaml")
	require.NoError(t, err)

	project := new(v1alpha3.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]any{
		"namespace": map[string]any{
			"labels":      map[string]any{"owner": clusterAdminPayload},
			"annotations": map[string]any{clusterAdminPayload: "value"},
		},
	}

	require.NoError(t, client.ensureParametersStayValues(project, template))

	manifests, err := client.renderTemplate(project, template)
	require.NoError(t, err)

	objects, err := objectDigests(manifests)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	for _, description := range objects {
		assert.Contains(t, description, "Namespace")
	}
}

// Parameters without a line break cannot produce structure, so the second render is skipped and the
// ordinary project pays nothing for the check.
func TestOrdinaryProjectSkipsTheSecondRender(t *testing.T) {
	assert.False(t, carriesLineBreak(map[string]any{"owner": "user@example.com"}))
	assert.True(t, carriesLineBreak(map[string]any{"owner": clusterAdminPayload}))

	// A line break spelled the way YAML also accepts it counts the same.
	assert.True(t, carriesLineBreak(map[string]any{"name": "evil\u2028---"}))
	assert.True(t, carriesLineBreak([]any{map[string]any{"name": "evil\r"}}))
}

func TestReplaceLineBreaksKeepsEverythingElse(t *testing.T) {
	value := map[string]any{
		"name":    "first\nsecond",
		"enabled": true,
		"count":   int64(3),
		"list":    []any{"a\rb"},
	}

	stripped, ok := replaceLineBreaks(value).(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "first second", stripped["name"])
	assert.Equal(t, true, stripped["enabled"])
	assert.Equal(t, int64(3), stripped["count"])
	assert.Equal(t, []any{"a b"}, stripped["list"].([]any))
	assert.False(t, strings.Contains(stripped["name"].(string), "\n"))
}
