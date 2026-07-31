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

func testClient(t *testing.T) *Client {
	t.Helper()

	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)

	return &Client{templates: templates, logger: ctrl.Log.WithName("test")}
}

func shippedTemplate(t *testing.T, name string) *v1alpha1.ProjectTemplate {
	t.Helper()

	template, err := read[v1alpha1.ProjectTemplate]("../../templates/" + name + ".yaml")
	require.NoError(t, err)
	require.NoError(t, validate.ProjectTemplate(template))

	return template
}

// stringParameterSchema declares one free-form string parameter, the shape a template author writes
// when the value is just text.
func stringParameterSchema(name string) map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			name: map[string]interface{}{"type": "string"},
		},
	}
}

// stripPattern removes the schema constraint on administrator names, leaving the template to answer
// for itself.
func stripPattern(template *v1alpha1.ProjectTemplate) {
	properties, _ := template.Spec.ParametersSchema.OpenAPIV3Schema["properties"].(map[string]interface{})
	administrators, _ := properties["administrators"].(map[string]interface{})
	items, _ := administrators["items"].(map[string]interface{})
	itemProperties, _ := items["properties"].(map[string]interface{})
	name, _ := itemProperties["name"].(map[string]interface{})
	delete(name, "pattern")
}

func projectWithAdministrator(name string, subject string) *v1alpha2.Project {
	project := new(v1alpha2.Project)
	project.Name = "test"
	project.Spec.ProjectTemplateName = "default"
	project.Spec.Parameters = map[string]interface{}{
		"resourceQuota": map[string]interface{}{
			"requests": map[string]interface{}{"cpu": "1"},
		},
		"administrators": []interface{}{
			map[string]interface{}{"subject": subject, "name": name},
		},
	}

	return project
}

// The schema is the first of the two barriers: a line break has no place in a subject name, and the
// value is refused before anything is rendered.
func TestAdministratorNameWithLineBreakIsRefusedBySchema(t *testing.T) {
	for _, name := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(name, func(t *testing.T) {
			template := shippedTemplate(t, name)

			project := projectWithAdministrator(clusterAdminPayload, "User")
			project.Spec.ProjectTemplateName = name

			err := validate.Project(project, template)
			require.Error(t, err, "the schema accepted a subject name carrying a line break")
			assert.Contains(t, err.Error(), "administrators")
		})
	}
}

// The second barrier is the template itself. Even with the schema out of the way -- a template
// written by a cluster administrator need not have one -- the value stays a value.
func TestAdministratorNameStaysAValue(t *testing.T) {
	client := testClient(t)
	template := shippedTemplate(t, "default")
	// The schema check is deliberately not exercised here: this is about what the template does with
	// a value the schema let through, which is the only thing standing behind a custom template.
	stripPattern(template)

	manifests, err := client.renderTemplate(projectWithAdministrator(clusterAdminPayload, "User"), template)
	require.NoError(t, err)

	// The payload is present -- as the administrator's name, which is where it was put. What matters
	// is that it produced no object of its own.
	objects, err := objectDigests(manifests)
	require.NoError(t, err)
	for _, description := range objects {
		assert.NotContains(t, description, "ClusterRoleBinding")
	}
}

// And the third one holds for a template nobody reviewed: the shipped ones are quoted now, a custom
// one need not be, so the check compares what the parameters actually rendered into.
func TestInjectedObjectIsRefusedForAnUnquotedTemplate(t *testing.T) {
	client := testClient(t)

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

	project := new(v1alpha2.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]interface{}{"owner": clusterAdminPayload}

	require.ErrorIs(t, client.ensureParametersStayValues(project, template), ErrParameterInjection)
}

// The same check with the injection kept small enough that the manifests parse either way: here the
// difference is not a parse error but an object that came out unlike the one the parameters describe.
func TestInjectedListItemIsRefusedAndNamed(t *testing.T) {
	client := testClient(t)

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

	project := new(v1alpha2.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]interface{}{"destination": "mine\n    - somebody-elses"}

	err := client.ensureParametersStayValues(project, template)
	require.ErrorIs(t, err, ErrParameterInjection)
	assert.Contains(t, err.Error(), "PodLoggingConfig deckhouse.io/v1alpha1/logs")
}

// A line break that stays inside a value is not injection, and refusing it would make the check
// unusable for templates that legitimately take a multi-line parameter.
func TestMultilineValueInAQuotedFieldIsAllowed(t *testing.T) {
	client := testClient(t)

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

	project := new(v1alpha2.Project)
	project.Name = "test"
	project.Spec.Parameters = map[string]interface{}{"script": "first line\nsecond line\n"}

	require.NoError(t, client.ensureParametersStayValues(project, template))
}

// Parameters without a line break cannot produce structure, so the second render is skipped and the
// ordinary project pays nothing for the check.
func TestOrdinaryProjectSkipsTheSecondRender(t *testing.T) {
	assert.False(t, carriesLineBreak(projectWithAdministrator("user@example.com", "User").Spec.Parameters))
	assert.True(t, carriesLineBreak(projectWithAdministrator(clusterAdminPayload, "User").Spec.Parameters))

	// A line break spelled the way YAML also accepts it counts the same.
	assert.True(t, carriesLineBreak(map[string]interface{}{"name": "evil\u2028---"}))
	assert.True(t, carriesLineBreak([]interface{}{map[string]interface{}{"name": "evil\r"}}))
}

func TestReplaceLineBreaksKeepsEverythingElse(t *testing.T) {
	value := map[string]interface{}{
		"name":    "first\nsecond",
		"enabled": true,
		"count":   int64(3),
		"list":    []interface{}{"a\rb"},
	}

	stripped, ok := replaceLineBreaks(value).(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "first second", stripped["name"])
	assert.Equal(t, true, stripped["enabled"])
	assert.Equal(t, int64(3), stripped["count"])
	assert.Equal(t, []interface{}{"a b"}, stripped["list"].([]interface{}))
	assert.False(t, strings.Contains(stripped["name"].(string), "\n"))
}
