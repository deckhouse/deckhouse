/*
Copyright 2024 Flant JSC

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

package validate

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

// schemaWithParameterFields is the parametersSchema of the default template from the time the
// administrators and the quota were template parameters, both required. Custom templates copied
// from it still carry it, while the Project now keeps the two in spec.administrators and spec.quota.
const schemaWithParameterFields = `
type: object
required: [administrators, resourceQuota]
properties:
  resourceQuota:
    type: object
    required: [requests]
    properties:
      requests:
        type: object
        anyOf:
          - {required: ["cpu"]}
          - {required: ["memory"]}
          - {required: ["storage"]}
        properties:
          cpu:
            oneOf:
              - {type: number, format: int}
              - {type: string}
            pattern: "^[0-9]+m?$"
          memory:
            oneOf:
              - {type: number, format: int}
              - {type: string}
            pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
          storage:
            type: string
            pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
      limits:
        type: object
        anyOf:
          - {required: ["cpu"]}
          - {required: ["memory"]}
        properties:
          cpu:
            oneOf:
              - {type: number, format: int}
              - {type: string}
            pattern: "^[0-9]+m?$"
          memory:
            oneOf:
              - {type: number, format: int}
              - {type: string}
            pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
  administrators:
    type: array
    items:
      type: object
      required: [subject, name]
      properties:
        subject: {type: string, enum: [User, Group]}
        name: {type: string, minLength: 1}
  networkPolicy:
    type: string
    enum: [Isolated, NotRestricted]
    default: Isolated
`

func templateWithSchema(t *testing.T, schema string) *v1alpha2.ProjectTemplate {
	t.Helper()
	openAPI := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(schema), &openAPI))
	return &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "custom"},
		Spec:       v1alpha2.ProjectTemplateSpec{ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: openAPI}},
	}
}

func TestProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		spec   v1alpha3.ProjectSpec
		// expectedErr is a part of the validation error; empty means the project is valid.
		expectedErr string
	}{
		{
			name:   "a schema with the parameter fields takes them from the standard fields",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "Group", Name: "team"}},
				Quota: corev1.ResourceList{
					"requests.cpu":     resource.MustParse("2"),
					"requests.memory":  resource.MustParse("4Gi"),
					"requests.storage": resource.MustParse("10Gi"),
					"limits.cpu":       resource.MustParse("1500m"),
					"limits.memory":    resource.MustParse("8Gi"),
				},
				Parameters: map[string]any{"networkPolicy": "Isolated"},
			},
		},
		{
			// a required field the standard fields do not have is validated in its empty form, and what
			// that form lacks is still refused
			name:        "a schema with the parameter fields still checks what they hold",
			schema:      schemaWithParameterFields,
			spec:        v1alpha3.ProjectSpec{Parameters: map[string]any{"networkPolicy": "Isolated"}},
			expectedErr: "resourceQuota.requests in body is required",
		},
		{
			// The conversion turns `administrators: []` into an empty spec.administrators, and the
			// controller writes the typed Project back without it (omitempty).
			name:   "an empty administrators list that went away on a write",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
			},
		},
		{
			// the canonical form of 1000 is 1k, which the cpu pattern of that schema refuses
			name:   "a whole cpu count keeps the form the schema takes",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Quota: corev1.ResourceList{
					"requests.cpu": resource.MustParse("1000"),
					"limits.cpu":   resource.MustParse("2000"),
				},
			},
		},
		{
			name: "a whole number stays a number for a branch that takes numbers only",
			schema: `
type: object
required: [resourceQuota]
properties:
  resourceQuota:
    type: object
    properties:
      requests:
        type: object
        properties:
          cpu:
            oneOf:
              - {type: integer}
              - {type: string, pattern: "^[0-9]+m$"}`,
			spec: v1alpha3.ProjectSpec{Quota: corev1.ResourceList{"requests.cpu": resource.MustParse("2")}},
		},
		{
			name:   "a quota given as a parameter is kept when the standard field is empty",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Parameters: map[string]any{
					"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "1"}},
				},
			},
		},
		{
			name:   "a schema with the parameter fields checks the standard fields against itself",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "ServiceAccount", Name: "robot"}},
				Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
			},
			expectedErr: "administrators.subject in body should be one of [User Group]",
		},
		{
			name:   "administrators given as a parameter are kept when the standard field is empty",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.memory": resource.MustParse("1Gi")},
				Parameters: map[string]any{
					"administrators": []any{map[string]any{"subject": "User", "name": "bob"}},
				},
			},
		},
		{
			name:   "a schema without the parameter fields does not get them",
			schema: "{type: object, properties: {networkPolicy: {type: string}}}",
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters:     map[string]any{"networkPolicy": "Isolated"},
			},
		},
		{
			name: "only the declared parameter field is taken",
			schema: `
type: object
required: [administrators]
properties:
  administrators:
    type: array
    items: {type: object, properties: {subject: {type: string}, name: {type: string}}}`,
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
			},
		},
		{
			name: "a quota value gets the type the schema declares for it",
			schema: `
type: object
required: [resourceQuota]
properties:
  resourceQuota:
    type: object
    properties:
      pods: {type: integer}
      requests: {type: object, properties: {storage: {type: string}}}
      limits: {type: object, properties: {cpu: {type: number}}}`,
			spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{
					"pods":             resource.MustParse("10"),
					"requests.storage": resource.MustParse("1073741824"),
					"limits.cpu":       resource.MustParse("500m"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}, Spec: tt.spec}
			parameters := runtime.DeepCopyJSON(tt.spec.Parameters)

			err := Project(project, templateWithSchema(t, tt.schema))

			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectedErr)
			}
			// the renderer reads the same parameters, so validation must leave them as they are
			assert.Equal(t, parameters, project.Spec.Parameters)
		})
	}
}

// legacyResourceQuota undoes what the v1alpha2 -> v1alpha3 conversion does to resourceQuota, the
// way webhooks/conversion/projects does it on the way back.
func TestLegacyResourceQuota(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		quota    corev1.ResourceList
		expected map[string]any
	}{
		{
			name: "requests and limits nest",
			quota: corev1.ResourceList{
				"requests.cpu":  resource.MustParse("1"),
				"limits.memory": resource.MustParse("2Gi"),
			},
			expected: map[string]any{
				"requests": map[string]any{"cpu": "1"},
				"limits":   map[string]any{"memory": "2Gi"},
			},
		},
		{
			name:     "a resource name with a dot is kept whole",
			quota:    corev1.ResourceList{"requests.nvidia.com/gpu": resource.MustParse("2")},
			expected: map[string]any{"requests": map[string]any{"nvidia.com/gpu": "2"}},
		},
		{
			name: "other keys stay flat",
			quota: corev1.ResourceList{
				"pods":                   resource.MustParse("10"),
				"count/deployments.apps": resource.MustParse("5"),
			},
			expected: map[string]any{"pods": "10", "count/deployments.apps": "5"},
		},
		{
			name: "a bare requests key gives way to a nested one",
			quota: corev1.ResourceList{
				"requests":     resource.MustParse("5"),
				"requests.cpu": resource.MustParse("1"),
			},
			expected: map[string]any{"requests": map[string]any{"cpu": "1"}},
		},
		{
			name:     "an empty quota",
			quota:    corev1.ResourceList{},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// the quota is a map, and the order of its keys changes from run to run
			for range 64 {
				assert.Equal(t, tt.expected, legacyResourceQuota(tt.quota, nil))
			}
		})
	}
}

// A Project read back after a write keeps an empty administrators list only as an absent field.
func TestProject_AfterAWrite(t *testing.T) {
	t.Parallel()

	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team"},
		Spec: v1alpha3.ProjectSpec{
			Administrators: []v1alpha3.Administrator{},
			Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
		},
	}
	raw, err := json.Marshal(project)
	require.NoError(t, err)
	written := new(v1alpha3.Project)
	require.NoError(t, json.Unmarshal(raw, written))
	require.Nil(t, written.Spec.Administrators, "the field is omitempty")

	require.NoError(t, Project(written, templateWithSchema(t, schemaWithParameterFields)))
}

func TestMergeWithDefaults_AdditionalProperties(t *testing.T) {
	schema := &spec.Schema{}
	schema.Properties = map[string]spec.Schema{
		"declared": {SchemaProps: spec.SchemaProps{Default: "from-schema"}},
	}
	schema.AdditionalProperties = &spec.SchemaOrBool{Allows: true}

	out := MergeDefaults(schema, map[string]any{"free": "value", "declared": "kept"})

	// the declared property keeps the project value, and the undeclared key is kept next to it.
	assert.Equal(t, map[string]any{"free": "value", "declared": "kept"}, out)
}

// TestMergeWithDefaults_ObjectDefaultPreserved pins the behaviour the schema-based ProjectTemplate
// render relies on: an object-typed property whose default is the whole object is kept verbatim (the
// merge only recurses into sub-properties when there is no default). Structured templates carry their
// fixed values (namespaceMetadata, dedicatedNodes, allowedUIDs) as such object defaults.
func TestMergeWithDefaults_ObjectDefaultPreserved(t *testing.T) {
	schema := &spec.Schema{}
	schema.Properties = map[string]spec.Schema{
		"namespace": {SchemaProps: spec.SchemaProps{
			Type:    spec.StringOrArray{"object"},
			Default: map[string]any{"labels": map[string]any{"team": "x"}},
			Properties: map[string]spec.Schema{
				"labels": {SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"object"}}},
			},
		}},
	}

	out := MergeDefaults(schema, map[string]any{})
	assert.Equal(t, map[string]any{"labels": map[string]any{"team": "x"}}, out["namespace"])
}
