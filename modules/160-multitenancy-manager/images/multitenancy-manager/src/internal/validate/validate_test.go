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
	"maps"
	"os"
	"path/filepath"
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
	"controller/internal/testutil"
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
			// the typed quota spells 2000m as 2 and 2048Mi as 2Gi, which the patterns refuse
			name: "a quota value takes the unit its schema pins",
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
          cpu: {type: string, pattern: "^[0-9]+m$"}
          memory: {type: string, pattern: "^[0-9]+Mi$"}
      limits:
        type: object
        properties:
          memory: {type: string, pattern: "^[0-9]+Ki$"}`,
			spec: v1alpha3.ProjectSpec{Quota: corev1.ResourceList{
				"requests.cpu":    resource.MustParse("2000m"),
				"requests.memory": resource.MustParse("2048Mi"),
				"limits.memory":   resource.MustParse("4Gi"),
			}},
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
			name:   "an additionalProperties schema that cannot hold the parameter fields does not get them",
			schema: "{type: object, properties: {owner: {type: string}}, additionalProperties: {type: string}}",
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters:     map[string]any{"owner": "alice", "team": "platform"},
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
		{
			// the conversion lifts no quota that holds the number 0
			name:   "a quota left in the parameters is not checked by a schema that does not declare it",
			schema: "{type: object, properties: {networkPolicy: {type: string}}}",
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Parameters: map[string]any{
					"networkPolicy": "Isolated",
					"resourceQuota": map[string]any{"requests": map[string]any{"cpu": float64(0), "memory": "2Gi"}},
				},
			},
		},
		{
			// the conversion lifts no empty administrators list
			name:   "administrators left in the parameters are not checked by a schema that does not declare them",
			schema: "{type: object, properties: {networkPolicy: {type: string}}}",
			spec: v1alpha3.ProjectSpec{
				Quota:      corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters: map[string]any{"administrators": []any{}},
			},
		},
		{
			name:   "additionalProperties false at the top declares neither of the two",
			schema: "{type: object, additionalProperties: false, properties: {networkPolicy: {type: string}}}",
			spec: v1alpha3.ProjectSpec{
				Parameters: map[string]any{
					"administrators": []any{},
					"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": float64(0)}},
				},
			},
		},
		{
			name:   "another parameter the schema does not declare is still refused",
			schema: "{type: object, properties: {networkPolicy: {type: string}}}",
			spec: v1alpha3.ProjectSpec{
				Parameters: map[string]any{"administrators": []any{}, "owner": "alice"},
			},
			expectedErr: "owner in body is a forbidden property",
		},
		{
			// the conversion lifts no quota value that is not a quantity
			name:   "a quota left in the parameters is checked by a schema that declares it",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
				Parameters: map[string]any{
					"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "lots"}},
				},
			},
			expectedErr: "resourceQuota.requests.cpu in body should match",
		},
		{
			name:   "administrators left in the parameters are checked by a schema that declares them",
			schema: schemaWithParameterFields,
			spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters: map[string]any{
					"administrators": []any{map[string]any{"subject": "ServiceAccount", "name": "robot"}},
				},
			},
			expectedErr: "administrators.subject in body should be one of [User Group]",
		},
		{
			name:   "administrators left in the parameters are checked by an additionalProperties schema at the top",
			schema: "{type: object, additionalProperties: {type: string}}",
			spec: v1alpha3.ProjectSpec{
				Parameters: map[string]any{"administrators": []any{}},
			},
			expectedErr: "administrators in body must be of type string",
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

// The built-in templates of the previous release declared administrators and resourceQuota, and the
// ones of this release declare neither. A project written on one of them in the v1alpha2 layout keeps
// in spec.parameters whatever of the two the v1alpha2 -> v1alpha3 conversion cannot give back as it
// was, and it has to validate after the update. The parameters are what the conversion makes of the
// projects e2e-bidef-zero and e2e-bidef-noadm of the upgrade e2e test.
func TestProject_BuiltinTemplatesTakeWhatTheConversionLeaves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec v1alpha3.ProjectSpec
	}{
		{
			name: "a quota with the number 0",
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}},
				Parameters: map[string]any{
					"networkPolicy": "Isolated",
					"resourceQuota": map[string]any{"requests": map[string]any{"cpu": float64(0), "memory": "2Gi"}},
				},
			},
		},
		{
			name: "an empty administrators list",
			spec: v1alpha3.ProjectSpec{
				Quota:      corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters: map[string]any{"administrators": []any{}},
			},
		},
	}

	for _, file := range []string{"default.yaml", "secure.yaml", "secure-with-dedicated-nodes.yaml"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "templates", file))
		require.NoError(t, err)
		template := new(v1alpha2.ProjectTemplate)
		require.NoError(t, yaml.Unmarshal(raw, template))

		for _, tt := range tests {
			t.Run(file+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}, Spec: *tt.spec.DeepCopy()}
				project.Spec.ProjectTemplateName = template.Name
				require.NoError(t, Project(project, template.DeepCopy()))
				assert.Equal(t, tt.spec.Parameters, project.Spec.Parameters, "validation leaves the parameters as they are")
			})
		}
	}
}

// loadSchema loads a parametersSchema written as YAML.
func loadSchema(t *testing.T, schema string) *spec.Schema {
	t.Helper()
	loaded, err := LoadSchema(templateWithSchema(t, schema).Spec.ParametersSchema.OpenAPIV3Schema)
	require.NoError(t, err)
	return loaded
}

// A schema declares a parameter by name or through an additionalProperties at the top that takes its
// value, and only a declared one is put back from its standard field.
func TestV1alpha2Parameters(t *testing.T) {
	t.Parallel()

	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team"},
		Spec: v1alpha3.ProjectSpec{
			Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("2")},
			Parameters:     map[string]any{"networkPolicy": "Isolated"},
		},
	}
	both := map[string]any{
		"networkPolicy":  "Isolated",
		"administrators": []any{map[string]any{"subject": "User", "name": "alice"}},
		"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "2"}},
	}

	tests := []struct {
		name     string
		schema   string
		expected map[string]any
	}{
		{
			name:     "declared by name",
			schema:   "{type: object, properties: {administrators: {type: array}, resourceQuota: {type: object}}}",
			expected: both,
		},
		{
			name:     "taken through additionalProperties at the top",
			schema:   "{type: object, additionalProperties: true}",
			expected: both,
		},
		{
			name:     "taken through an additionalProperties schema at the top",
			schema:   "{type: object, additionalProperties: {x-kubernetes-preserve-unknown-fields: true}}",
			expected: both,
		},
		{
			name:     "refused through additionalProperties at the top",
			schema:   "{type: object, additionalProperties: false, properties: {networkPolicy: {type: string}}}",
			expected: project.Spec.Parameters,
		},
		{
			// a v1alpha2 project on that template could hold neither
			name:     "not taken by an additionalProperties schema at the top",
			schema:   "{type: object, additionalProperties: {type: string}}",
			expected: project.Spec.Parameters,
		},
		{
			name:   "only the one an additionalProperties schema at the top takes",
			schema: "{type: object, additionalProperties: {type: object}}",
			expected: map[string]any{
				"networkPolicy": "Isolated",
				"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "2"}},
			},
		},
		{
			name:     "not declared",
			schema:   "{type: object, properties: {networkPolicy: {type: string}}}",
			expected: project.Spec.Parameters,
		},
		{
			name:   "a cpu count in the unit the schema pins",
			schema: "{type: object, properties: {resourceQuota: {type: object, properties: {requests: {type: object, properties: {cpu: {type: string, pattern: '^[0-9]+m$'}}}}}}}",
			expected: map[string]any{
				"networkPolicy": "Isolated",
				"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "2000m"}},
			},
		},
		{
			// the cpu count is a JSON number for a schema that takes no string
			name:   "one declared by name",
			schema: "{type: object, properties: {resourceQuota: {type: object, properties: {requests: {type: object, properties: {cpu: {type: integer}}}}}}}",
			expected: map[string]any{
				"networkPolicy": "Isolated",
				"resourceQuota": map[string]any{"requests": map[string]any{"cpu": int64(2)}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, V1alpha2Parameters(project, loadSchema(t, tt.schema)))
		})
	}

	assert.Equal(t, project.Spec.Parameters, V1alpha2Parameters(project, nil), "a schema that did not load")
}

// V1alpha2Parameters is the Go copy of what the v1alpha3 -> v1alpha2 conversion of
// webhooks/conversion/projects gives a v1alpha2 reader, and a Helm text renders from it. The hooks
// test TestProjectDownConversionParameters runs that conversion over the projects of a golden fixture
// with every jq engine, this runs the copy over their specs, and both compare with the parameters the
// fixture converts them to. The schemas here declare the two parameters without requiring them. A
// required one the project leaves empty is put back in its empty form, which the conversion does not
// do (see TestProject), so the schema of the default template of that layout, which requires both, is
// used only for the specs that have both standard fields.
func TestV1alpha2ParametersFollowTheConversion(t *testing.T) {
	t.Parallel()

	fixture := testutil.ModuleFile(t, "hooks/testdata/conversion/projects/parameters/v1alpha3_to_v1alpha2.yaml")
	raw, err := os.ReadFile(fixture)
	require.NoError(t, err)

	// A case of the fixture is one ConversionReview of the hook.
	var cases []struct {
		Name      string           `json:"name"`
		Objects   []map[string]any `json:"objects"`
		Converted []map[string]any `json:"converted"`
	}
	require.NoError(t, yaml.UnmarshalStrict(raw, &cases))
	require.NotEmpty(t, cases)

	const quotaValue = "{anyOf: [{type: integer}, {type: string}]}"
	schemas := map[string]string{
		"declared by name": `
type: object
properties:
  administrators:
    type: array
    items: {type: object, properties: {subject: {type: string}, name: {type: string}}}
  resourceQuota:
    type: object
    additionalProperties: ` + quotaValue + `
    properties:
      requests: {type: object, additionalProperties: ` + quotaValue + `}
      limits: {type: object, additionalProperties: ` + quotaValue + `}
  networkPolicy: {type: string}`,
		"taken through additionalProperties at the top": "{type: object, additionalProperties: true}",
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			require.Len(t, tc.Converted, len(tc.Objects), "a converted object per object")

			for i, object := range tc.Objects {
				stored, err := json.Marshal(object["spec"])
				require.NoError(t, err)
				project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}}
				require.NoError(t, json.Unmarshal(stored, &project.Spec))
				written, err := json.Marshal(project.Spec)
				require.NoError(t, err)
				require.JSONEq(t, string(stored), string(written), "the spec is not what a typed write stores")

				converted, _ := tc.Converted[i]["spec"].(map[string]any)
				want, err := json.Marshal(converted["parameters"])
				require.NoError(t, err)

				checked := maps.Clone(schemas)
				if project.Spec.Administrators != nil && project.Spec.Quota != nil {
					checked["of the default template of that layout"] = schemaWithParameterFields
				}
				for name, schema := range checked {
					got, err := json.Marshal(V1alpha2Parameters(project, loadSchema(t, schema)))
					require.NoError(t, err)
					assert.JSONEq(t, string(want), string(got), name)
				}
			}
		})
	}
}

// A schema leaves administrators or resourceQuota undeclared when it neither names it nor has a
// top-level additionalProperties other than false. An additionalProperties schema declares both
// whatever it takes, and validation then checks them against it. A null counts as held.
func TestUndeclaredLeftovers(t *testing.T) {
	t.Parallel()

	both := map[string]any{"administrators": []any{}, "resourceQuota": nil, "networkPolicy": "Isolated"}
	tests := []struct {
		name       string
		schema     string
		parameters map[string]any
		expected   []string
	}{
		{
			name:       "a schema that names neither",
			schema:     "{type: object, properties: {networkPolicy: {type: string}}}",
			parameters: both,
			expected:   []string{"administrators", "resourceQuota"},
		},
		{
			name:       "additionalProperties false at the top",
			schema:     "{type: object, additionalProperties: false, properties: {networkPolicy: {type: string}}}",
			parameters: both,
			expected:   []string{"administrators", "resourceQuota"},
		},
		{
			name:       "a schema that names one",
			schema:     "{type: object, properties: {resourceQuota: {type: object}}}",
			parameters: both,
			expected:   []string{"administrators"},
		},
		{
			name:       "an additionalProperties schema at the top that takes neither",
			schema:     "{type: object, additionalProperties: {type: string}}",
			parameters: both,
		},
		{
			name:       "additionalProperties true at the top",
			schema:     "{type: object, additionalProperties: true}",
			parameters: both,
		},
		{
			name:       "parameters that hold neither",
			schema:     "{type: object, properties: {networkPolicy: {type: string}}}",
			parameters: map[string]any{"networkPolicy": "Isolated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, UndeclaredLeftovers(tt.parameters, loadSchema(t, tt.schema)))
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

// A quantity is tried in its canonical spelling first, then as a number and in every whole spelling
// with a unit, since the parameter may have held any of them and a schema may take only one.
func TestQuotaValueForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		quantity string
		expected []any
	}{
		{quantity: "2", expected: []any{"2", int64(2), "2000m"}},
		{quantity: "2000m", expected: []any{"2", int64(2), "2000m"}},
		{quantity: "1500m", expected: []any{"1500m", 1.5}},
		{quantity: "2000", expected: []any{"2k", int64(2000), "2000", "2000000m"}},
		{quantity: "1Ki", expected: []any{"1Ki", int64(1024), "1024", "1024000m"}},
		{
			quantity: "2048Mi",
			expected: []any{"2Gi", int64(2147483648), "2147483648", "2147483648000m", "2048Mi", "2097152Ki"},
		},
		{
			quantity: "1Ei",
			expected: []any{"1Ei", int64(1 << 60), "1152921504606846976", "1024Pi", "1048576Ti", "1073741824Gi", "1099511627776Mi", "1125899906842624Ki"},
		},
		{quantity: "3G", expected: []any{"3G", int64(3e9), "3000000000", "3000000000000m", "3000M", "3000000k"}},
		{quantity: "0", expected: []any{"0", int64(0), "0m"}},
	}

	for _, tt := range tests {
		t.Run(tt.quantity, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, quotaValueForms(resource.MustParse(tt.quantity)))
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
