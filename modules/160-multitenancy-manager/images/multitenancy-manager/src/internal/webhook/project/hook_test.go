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

package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	projectmanager "controller/internal/manager/project"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

func TestValidateStandardFields(t *testing.T) {
	bareMemory := corev1.ResourceList{"requests.memory": resource.MustParse("1073741824")}
	stale := []v1alpha3.Administrator{{Kind: "ServiceAccount", Name: "robot"}}
	bothAdministrators := v1alpha3.ProjectSpec{
		Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
		Parameters:     map[string]any{"administrators": []any{"bob"}},
	}
	bothQuotas := v1alpha3.ProjectSpec{
		Quota:      corev1.ResourceList{"pods": resource.MustParse("10")},
		Parameters: map[string]any{"resourceQuota": "10Gi"},
	}

	cases := []struct {
		name           string
		administrators []v1alpha3.Administrator
		// quota is spec.quota as the request writes it: a Go string is a JSON string, a Go int a
		// JSON number.
		quota      map[string]any
		parameters map[string]any
		// old is the stored project of an update; nil checks the project as a create.
		old *v1alpha3.Project
		// v1alpha2Create checks the project as a create through deckhouse.io/v1alpha2.
		v1alpha2Create bool
		denied         bool
		warned         bool
	}{
		{
			name: "empty is valid",
		},
		{
			name:           "valid administrators and quota",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "Group", Name: "team"}},
			quota: map[string]any{
				"requests.cpu":    "2",
				"requests.memory": "1Gi",
				"limits.memory":   "512Mi",
				"pods":            "10",
			},
		},
		{
			name: "cpu and pods may be bare numbers",
			quota: map[string]any{
				"requests.cpu": 5,
				"limits.cpu":   "500m",
				"pods":         "10",
			},
		},
		{
			name:  "memory with decimal SI unit is allowed",
			quota: map[string]any{"requests.memory": "1G"},
		},
		{
			name:   "bare memory is denied",
			quota:  map[string]any{"requests.memory": "5"},
			denied: true,
		},
		{
			name:   "bare storage is denied",
			quota:  map[string]any{"requests.storage": "5"},
			denied: true,
		},
		{
			name:   "ephemeral-storage without unit is denied",
			quota:  map[string]any{"requests.ephemeral-storage": "100"},
			denied: true,
		},
		{
			name:   "memory milli suffix is denied",
			quota:  map[string]any{"limits.memory": "1000m"},
			denied: true,
		},
		{
			// the canonical form of 1000000 is 1M, which ends in a unit
			name:   "a bare multiple of a power of 1000 is denied",
			quota:  map[string]any{"requests.memory": "1000000"},
			denied: true,
		},
		{
			name:   "a bare JSON number is denied",
			quota:  map[string]any{"requests.storage": 1000000},
			denied: true,
		},
		{
			name:  "a zero memory or storage needs no unit",
			quota: map[string]any{"requests.storage": "0", "limits.memory": 0},
		},
		{
			name:   "bare memory the stored project already holds",
			quota:  map[string]any{"requests.memory": "1073741824"},
			old:    &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Quota: bareMemory}},
			warned: true,
		},
		{
			name:   "bare memory the stored project holds in another spelling",
			quota:  map[string]any{"requests.memory": "1073741824000m"},
			old:    &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Quota: bareMemory}},
			warned: true,
		},
		{
			name: "bare memory next to a key the update changes",
			quota: map[string]any{
				"requests.memory": 1073741824,
				"requests.cpu":    "4",
			},
			old:    &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Quota: bareMemory}},
			warned: true,
		},
		{
			name:   "bare memory an update sets",
			quota:  map[string]any{"requests.memory": "2147483648"},
			old:    &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Quota: bareMemory}},
			denied: true,
		},
		{
			name: "bare storage an update adds",
			quota: map[string]any{
				"requests.memory":  "1073741824",
				"requests.storage": "10",
			},
			old:    &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Quota: bareMemory}},
			denied: true,
		},
		{
			// a manifest of the previous release, created again from a backup or by a GitOps tool
			name: "bare memory and storage of a create through v1alpha2 pass with a warning",
			quota: map[string]any{
				"requests.memory":  5368709120,
				"requests.storage": "10737418240",
			},
			v1alpha2Create: true,
			warned:         true,
		},
		{
			name:           "a unit a create through v1alpha2 writes needs no warning",
			quota:          map[string]any{"requests.memory": "5Gi"},
			v1alpha2Create: true,
		},
		{
			name:  "hugepages count may be bare",
			quota: map[string]any{"requests.hugepages-2Mi": "5"},
		},
		{
			// the conversion carries bare numbers of the resourceQuota parameter over into spec.quota
			name: "bare memory and storage kept unchanged by an update pass with a warning",
			quota: map[string]any{
				"requests.memory":  "5368709120",
				"requests.storage": 10737418240,
			},
			old: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Quota: corev1.ResourceList{
				"requests.memory":  resource.MustParse("5368709120"),
				"requests.storage": resource.MustParse("10737418240"),
			}}},
			warned: true,
		},
		{
			name:  "a bare multiple of a power of 1000 kept unchanged by an update passes with a warning",
			quota: map[string]any{"requests.memory": 1000},
			old: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.memory": resource.MustParse("1000")},
			}},
			warned: true,
		},
		{
			name:  "bare memory changed by an update is denied",
			quota: map[string]any{"limits.memory": "6"},
			old: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"limits.memory": resource.MustParse("5")},
			}},
			denied: true,
		},
		{
			name: "bare memory added by an update is denied",
			quota: map[string]any{
				"requests.cpu":    "1",
				"requests.memory": "5",
			},
			old: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
			}},
			denied: true,
		},
		{
			name:  "bare memory lowered by an update is denied",
			quota: map[string]any{"limits.memory": "4294967296"},
			old: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"limits.memory": resource.MustParse("5368709120")},
			}},
			denied: true,
		},
		{
			// 0 reads the same in every unit, so it needs none even under a key the stored quota lacks
			name:  "a bare zero added by an update needs no unit",
			quota: map[string]any{"requests.memory": "0"},
			old: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
			}},
		},
		{
			name:           "invalid administrator kind",
			administrators: []v1alpha3.Administrator{{Kind: "ServiceAccount", Name: "robot"}},
			denied:         true,
		},
		{
			name:           "empty administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: ""}},
			denied:         true,
		},
		{
			name:           "an administrator the stored project already has",
			administrators: append([]v1alpha3.Administrator{{Kind: "User", Name: "alice"}}, stale...),
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Administrators: stale}},
		},
		{
			name:           "an invalid administrator an update adds",
			administrators: append([]v1alpha3.Administrator{{Kind: "Robot", Name: "r2"}}, stale...),
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Administrators: stale}},
			denied:         true,
		},
		{
			name:           "administrators next to the parameter they replace",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			parameters:     map[string]any{"administrators": []any{"bob"}},
			denied:         true,
		},
		{
			name:           "an empty administrators list next to the parameter",
			administrators: []v1alpha3.Administrator{},
			parameters:     map[string]any{"administrators": []any{map[string]any{"subject": "User", "name": "bob"}}},
			denied:         true,
		},
		{
			name:       "quota next to the parameter it replaces",
			quota:      map[string]any{"pods": "10"},
			parameters: map[string]any{"resourceQuota": map[string]any{"pods": "1", "scopes": []any{"BestEffort"}}},
			denied:     true,
		},
		{
			name:       "an empty quota next to the parameter",
			quota:      map[string]any{},
			parameters: map[string]any{"resourceQuota": "10Gi"},
			denied:     true,
		},
		{
			name:           "administrators next to the parameter as the stored project has them",
			administrators: bothAdministrators.Administrators,
			parameters:     bothAdministrators.Parameters,
			old:            &v1alpha3.Project{Spec: *bothAdministrators.DeepCopy()},
		},
		{
			name:       "a quota next to the parameter as the stored project has them",
			quota:      map[string]any{"pods": "10"},
			parameters: bothQuotas.Parameters,
			old:        &v1alpha3.Project{Spec: *bothQuotas.DeepCopy()},
		},
		{
			name:           "administrators an update changes next to the parameter",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "carol"}},
			parameters:     bothAdministrators.Parameters,
			old:            &v1alpha3.Project{Spec: *bothAdministrators.DeepCopy()},
			denied:         true,
		},
		{
			name:       "a parameter an update changes next to the quota",
			quota:      map[string]any{"pods": "10"},
			parameters: map[string]any{"resourceQuota": "20Gi"},
			old:        &v1alpha3.Project{Spec: *bothQuotas.DeepCopy()},
			denied:     true,
		},
		{
			// nil and an empty list compare equal, and only one of them is set
			name:           "an empty administrators list an update adds next to the parameter",
			administrators: []v1alpha3.Administrator{},
			parameters:     map[string]any{"administrators": []any{"bob"}},
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Parameters: map[string]any{"administrators": []any{"bob"}}}},
			denied:         true,
		},
		{
			name:           "standard fields next to null parameters",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			quota:          map[string]any{"pods": "10"},
			parameters:     map[string]any{"administrators": nil, "resourceQuota": nil},
		},
		{
			name:       "parameters without standard fields",
			parameters: map[string]any{"administrators": []any{"bob"}, "resourceQuota": "10Gi"},
		},
		{
			name:           "administrators next to a quota left in the parameters",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			parameters:     map[string]any{"resourceQuota": "10Gi"},
		},
		{
			name:           "a line separator in an administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u2028x"}},
			denied:         true,
		},
		{
			name:           "a paragraph separator in an administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "Group", Name: "team\u2029x"}},
			denied:         true,
		},
		{
			name:           "a next line character in an administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u0085x"}},
			denied:         true,
		},
		{
			name:           "a C1 control in an administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u0080x"}},
			denied:         true,
		},
		{
			name:           "a control sequence introducer in an administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "Group", Name: "team\u009bx"}},
			denied:         true,
		},
		{
			name:           "a no-break space in an administrator name",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u00a0x"}},
		},
		{
			// the conversion lifts a name the schema of a v1alpha2 template let in
			name:           "a line separator in a name an update keeps passes with a warning",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "bob"}, {Kind: "User", Name: "alice\u2028x"}},
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u2028x"}}}},
			warned:         true,
		},
		{
			name:           "a line separator in a name an update adds is denied",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "bob"}, {Kind: "User", Name: "alice\u2028x"}},
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "bob"}}}},
			denied:         true,
		},
		{
			// the stored project has the name, but not this administrator
			name:           "a line separator in a name an update moves to another kind is denied",
			administrators: []v1alpha3.Administrator{{Kind: "Group", Name: "alice\u2028x"}},
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u2028x"}}}},
			denied:         true,
		},
		{
			name:           "a kept name does not let a bare memory value through",
			administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u2028x"}},
			quota:          map[string]any{"requests.memory": "5"},
			old:            &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice\u2028x"}}}},
			denied:         true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"spec": map[string]any{
				"administrators": tc.administrators,
				"quota":          tc.quota,
				"parameters":     tc.parameters,
			}})
			require.NoError(t, err)
			project := new(v1alpha3.Project)
			require.NoError(t, json.Unmarshal(raw, project))
			spellings, err := quotaSpellings(raw)
			require.NoError(t, err)

			msg, warnings := validateStandardFields(project, spellings, tc.old, tc.v1alpha2Create)
			if tc.denied {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}
			if tc.warned {
				assert.NotEmpty(t, warnings)
			} else {
				assert.Empty(t, warnings)
			}
		})
	}
}

// The unit rule reads a quota value as the request writes it. The parsed Quantity would not do: its
// canonical form of a bare 1000 is 1k and of a bare 5000000 is 5M, and both end in a unit.
func TestHasByteUnitSuffix(t *testing.T) {
	cases := []struct {
		name    string
		value   string // the JSON of a spec.quota value
		hasUnit bool
	}{
		{name: "a string of digits", value: `"1000"`},
		{name: "a string of digits that reads 5M in canonical form", value: `"5000000"`},
		{name: "a lowercase exponent", value: `"1e3"`},
		{name: "an uppercase exponent", value: `"1E6"`},
		{name: "a string of digits that is no multiple of 1000", value: `"2048"`},
		{name: "a JSON number", value: `1000`},
		{name: "a JSON number that is no multiple of 1000", value: `2048`},
		{name: "a decimal kilo", value: `"1k"`, hasUnit: true},
		{name: "a binary kilo", value: `"1Ki"`, hasUnit: true},
		{name: "a decimal mega", value: `"5M"`, hasUnit: true},
		{name: "a binary mega", value: `"512Mi"`, hasUnit: true},
		{name: "a fraction of a binary giga", value: `"1.5Gi"`, hasUnit: true},
		{name: "a unit with spaces around it, which Quantity trims", value: `" 5Gi "`, hasUnit: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spellings, err := quotaSpellings([]byte(`{"spec":{"quota":{"requests.memory":` + tc.value + `}}}`))
			require.NoError(t, err)
			assert.Equal(t, tc.hasUnit, hasByteUnitSuffix(spellings["requests.memory"]))
		})
	}
}

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	// the chart the controller ships, so a test renders what the webhook renders in the cluster
	helmClient, err := helm.NewRenderOnly("../../../helmlib", logr.Discard())
	require.NoError(t, err)
	return &validator{client: newClientBuilder(t, objs...).Build(), helmClient: helmClient}
}

func newClientBuilder(t *testing.T, objs ...client.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, authorizationv1.AddToScheme, v1alpha2.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...)
}

// The template is read at v1alpha2, and projected onto the legacy shape only for the Helm render. A
// read at v1alpha1 loses the structured fields, and while that version was not served every project
// write was denied with "no matches for kind ProjectTemplate in version deckhouse.io/v1alpha1".
func TestProjectTemplateByName(t *testing.T) {
	schema := map[string]any{"type": "object"}
	stored := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "simple"},
		Spec: v1alpha2.ProjectTemplateSpec{
			Description:      "a template",
			ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: schema},
		},
	}

	v := newValidator(t, stored)

	template, err := v.projectTemplateByName(context.Background(), "simple")
	require.NoError(t, err)
	require.NotNil(t, template, "the template was not found at the storage version")
	assert.Equal(t, "a template", template.Spec.Description)
	assert.Equal(t, schema, template.Spec.ParametersSchema.OpenAPIV3Schema)

	missing, err := v.projectTemplateByName(context.Background(), "absent")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func projectWithParameters(parameters map[string]any) *v1alpha3.Project {
	return &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec:       v1alpha3.ProjectSpec{Parameters: parameters},
	}
}

func updateRequest(t *testing.T, user string, old, updated *v1alpha3.Project) admission.Request {
	t.Helper()
	oldRaw, err := json.Marshal(old)
	require.NoError(t, err)
	newRaw, err := json.Marshal(updated)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		UserInfo:  authnv1.UserInfo{Username: user},
		Object:    runtime.RawExtension{Raw: newRaw},
		OldObject: runtime.RawExtension{Raw: oldRaw},
	}}
}

// A project is the source of truth for its namespace now, so its spec is an ordinary user-editable
// field: the namespace no longer owns it and there is nothing to protect it from.
func TestHandle_ProjectSpecIsUserEditable(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()

	old := projectWithParameters(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "1"}}})
	updated := projectWithParameters(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "2"}}})

	resp := v.Handle(ctx, updateRequest(t, "alice", old, updated))
	assert.True(t, resp.Allowed)
}

// The down-conversion writes a standard field over the parameter it was lifted from, so a v1alpha3
// client that sets both is refused on create and on update, before the template is even looked up.
func TestHandle_StandardFieldNextToItsParameter(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()

	old := projectWithParameters(map[string]any{"administrators": []any{"bob"}})
	updated := old.DeepCopy()
	updated.Spec.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice"}}

	resp := v.Handle(ctx, updateRequest(t, "alice", old, updated))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "spec.administrators and spec.parameters.administrators cannot both be set")

	created := projectWithParameters(map[string]any{"resourceQuota": map[string]any{"requests": map[string]any{"cpu": "1"}}})
	created.Spec.Quota = corev1.ResourceList{"requests.cpu": resource.MustParse("1")}
	raw, err := json.Marshal(created)
	require.NoError(t, err)
	resp = v.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		UserInfo:  authnv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: raw},
	}})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "spec.quota and spec.parameters.resourceQuota cannot both be set")

	// the parameter alone is allowed
	resp = v.Handle(ctx, updateRequest(t, "alice", old, old))
	assert.True(t, resp.Allowed, resp.Result)
}

// The up-conversion lifts what an older release accepted, such as a memory quota without a unit, into
// the standard fields. An update of anything else has to go through, and only what an update changes
// is checked. A stored project that does not decode leaves the update to be checked in full. A user's
// update decodes the stored project earlier, for the checks of the project's own labels, so the case
// is one of the controller.
func TestHandle_UpdateKeepsWhatTheStoredProjectHolds(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()

	old := projectWithParameters(map[string]any{"networkPolicy": "Isolated"})
	old.Spec.Quota = corev1.ResourceList{"requests.memory": resource.MustParse("1073741824")}
	edited := old.DeepCopy()
	edited.Spec.Description = "edited"
	edited.Labels = map[string]string{"team": "backend"}

	resp := v.Handle(ctx, updateRequest(t, "alice", old, edited))
	assert.True(t, resp.Allowed, resp.Result)
	require.Len(t, resp.Warnings, 1)
	assert.Contains(t, resp.Warnings[0], "requests.memory is 1073741824 without a unit suffix")

	changed := edited.DeepCopy()
	changed.Spec.Quota["requests.memory"] = resource.MustParse("2147483648")
	resp = v.Handle(ctx, updateRequest(t, "alice", old, changed))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "requests.memory must include a unit suffix")

	undecodable := updateRequest(t, rolebindingwebhook.ControllerServiceAccount, old, edited)
	undecodable.OldObject.Raw = []byte(`{"metadata":{"name":"foo"},"spec":{"quota":{"requests.memory":"lots"}}}`)
	resp = v.Handle(ctx, undecodable)
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "requests.memory must include a unit suffix")
}

// A project written with administrators and resourceQuota in its parameters, on a template whose
// schema requires both, comes out of the conversion with the two in its standard fields and with
// the bare byte counts it was written with. Writing it back unchanged, as a GitOps re-apply does,
// has to pass; a new bare byte count does not.
func TestHandle_UpdateOfAProjectFromTheParametersLayout(t *testing.T) {
	const schema = `
type: object
required: [administrators, resourceQuota]
properties:
  administrators:
    type: array
    items:
      type: object
      required: [subject, name]
      properties:
        subject: {type: string, enum: [User, Group]}
        name: {type: string, minLength: 1}
  resourceQuota:
    type: object
    properties:
      requests:
        type: object
        properties:
          memory:
            oneOf:
              - {type: number}
              - {type: string}
`
	openAPI := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(schema), &openAPI))
	template := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "custom"},
		Spec:       v1alpha2.ProjectTemplateSpec{ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: openAPI}},
	}
	old := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team"},
		Spec: v1alpha3.ProjectSpec{
			ProjectTemplateName: "custom",
			Administrators:      []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			Quota:               corev1.ResourceList{"requests.memory": resource.MustParse("5368709120")},
		},
	}
	v := newValidator(t, template)

	t.Run("the unchanged project passes with a warning", func(t *testing.T) {
		resp := v.Handle(context.Background(), updateRequest(t, "alice", old, old.DeepCopy()))
		assert.True(t, resp.Allowed, resp.Result)
		assert.NotEmpty(t, resp.Warnings)
	})

	t.Run("a new bare byte count is denied", func(t *testing.T) {
		updated := old.DeepCopy()
		updated.Spec.Quota["requests.memory"] = resource.MustParse("6442450944")
		resp := v.Handle(context.Background(), updateRequest(t, "alice", old, updated))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "requests.memory must include a unit suffix")
	})

	t.Run("a byte count with a unit passes without a warning", func(t *testing.T) {
		updated := old.DeepCopy()
		updated.Spec.Quota["requests.memory"] = resource.MustParse("6Gi")
		resp := v.Handle(context.Background(), updateRequest(t, "alice", old, updated))
		assert.True(t, resp.Allowed, resp.Result)
		assert.Empty(t, resp.Warnings)
	})
}

// The unit rule reads spec.quota as the request writes it, a JSON number or a string, and not as
// the parsed Quantity prints it: there a bare 1000000 reads 1M and a bare 1000 reads 1k. A request
// that writes such a value anew is refused, and one that leaves it as it was passes with a warning
// that names the value as written. A bare 0 reads the same in every unit and needs none.
func TestHandle_BareByteQuotaAsWritten(t *testing.T) {
	request := func(operation admissionv1.Operation, old, updated string) admission.Request {
		req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: operation,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: []byte(updated)},
		}}
		if old != "" {
			req.OldObject = runtime.RawExtension{Raw: []byte(old)}
		}
		return req
	}
	project := func(memory string) string {
		return `{"metadata":{"name":"team"},"spec":{"quota":{"requests.memory":` + memory + `}}}`
	}
	v := newValidator(t)

	for _, memory := range []string{`1000000`, `"1000000"`} {
		t.Run("a new bare "+memory+" is refused", func(t *testing.T) {
			resp := v.Handle(context.Background(), request(admissionv1.Create, "", project(memory)))
			assert.False(t, resp.Allowed)
			assert.Contains(t, resp.Result.Message, "requests.memory must include a unit suffix")

			resp = v.Handle(context.Background(), request(admissionv1.Update, project(`"1Gi"`), project(memory)))
			assert.False(t, resp.Allowed)
			assert.Contains(t, resp.Result.Message, "requests.memory must include a unit suffix")
		})
	}

	t.Run("an unchanged bare 1000 passes with a warning", func(t *testing.T) {
		resp := v.Handle(context.Background(), request(admissionv1.Update, project(`1000`), project(`1000`)))
		assert.True(t, resp.Allowed, resp.Result)
		require.Len(t, resp.Warnings, 1)
		assert.Contains(t, resp.Warnings[0], "requests.memory is 1000 without a unit suffix")
	})

	t.Run("a unit passes without a warning", func(t *testing.T) {
		resp := v.Handle(context.Background(), request(admissionv1.Update, project(`1000`), project(`"1k"`)))
		assert.True(t, resp.Allowed, resp.Result)
		assert.Empty(t, resp.Warnings)
	})

	for _, memory := range []string{`0`, `"0"`} {
		t.Run("a new bare "+memory+" passes without a warning", func(t *testing.T) {
			resp := v.Handle(context.Background(), request(admissionv1.Create, "", project(memory)))
			assert.True(t, resp.Allowed, resp.Result)
			assert.Empty(t, resp.Warnings)
		})
	}

	// The resourceQuota parameter of a v1alpha2 project took bare numbers, and the conversion lifts
	// them into spec.quota as they are. A create through v1alpha2, as a restore from a backup or a
	// GitOps tool that creates the project again writes it, passes with a warning. An update through
	// v1alpha2 that sets a new one does not.
	throughV1alpha2 := func(req admission.Request) admission.Request {
		req.RequestKind = &metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha2", Kind: "Project"}
		return req
	}
	for _, memory := range []string{`1000000`, `"1000000"`} {
		t.Run("a bare "+memory+" a create through v1alpha2 writes passes with a warning", func(t *testing.T) {
			resp := v.Handle(context.Background(), throughV1alpha2(request(admissionv1.Create, "", project(memory))))
			assert.True(t, resp.Allowed, resp.Result)
			require.Len(t, resp.Warnings, 1)
			assert.Contains(t, resp.Warnings[0], "requests.memory is 1000000 without a unit suffix")
			assert.Contains(t, resp.Warnings[0], "created through deckhouse.io/v1alpha2")

			resp = v.Handle(context.Background(), throughV1alpha2(request(admissionv1.Update, project(`"1Gi"`), project(memory))))
			assert.False(t, resp.Allowed)
			assert.Contains(t, resp.Result.Message, "requests.memory must include a unit suffix")
		})
	}
}

func TestHandle_CreateOverExistingNamespace(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	v := newValidator(t, ns)
	ctx := context.Background()

	createReq := func(user string, labels map[string]string) admission.Request {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: labels}}
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: user},
			Object:    runtime.RawExtension{Raw: raw},
		}}
	}

	// the controller adopts an existing namespace into a project of its own
	resp := v.Handle(ctx, createReq(rolebindingwebhook.ControllerServiceAccount, nil))
	assert.True(t, resp.Allowed)

	// a regular user creating a project that collides with an existing namespace is denied
	resp = v.Handle(ctx, createReq("alice", nil))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "a namespace with its name exists")
}

// The name rules the webhook enforces on create. The platform prefixes are reserved. Of the
// "<project>-*" names only the ones a ProjectNamespace has claimed are taken: any other such name is
// a separate project, and the webhook lets it through with a warning that says so, because it reads
// like an additional namespace of the other project and is not one.
func TestHandle_ProjectNameValidation(t *testing.T) {
	createReq := func(t *testing.T, name string) admission.Request {
		t.Helper()
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}}
	}
	project := func(name string) *v1alpha3.Project {
		return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}

	t.Run("platform prefixes are reserved", func(t *testing.T) {
		v := newValidator(t)
		for _, name := range []string{"d8-system", "d8-foo", "kube-system", "kube-foo"} {
			resp := v.Handle(context.Background(), createReq(t, name))
			assert.False(t, resp.Allowed, name)
			assert.Contains(t, resp.Result.Message, "cannot start with 'd8-' or 'kube-'", name)
		}
	})

	t.Run("a name under an existing project's name is a separate project, with a warning", func(t *testing.T) {
		v := newValidator(t, project("foo"))
		resp := v.Handle(context.Background(), createReq(t, "foo-bar"))
		require.True(t, resp.Allowed, resp.Result)
		require.Len(t, resp.Warnings, 1)
		assert.Contains(t, resp.Warnings[0], `Project "foo-bar" is not an additional namespace of project "foo"`)
		assert.Contains(t, resp.Warnings[0], `create a ProjectNamespace in the "foo" namespace`)
	})

	t.Run("every project whose name the new one extends is named", func(t *testing.T) {
		v := newValidator(t, project("foo-bar"), project("foo"))
		resp := v.Handle(context.Background(), createReq(t, "foo-bar-baz"))
		require.True(t, resp.Allowed, resp.Result)
		require.Len(t, resp.Warnings, 2)
		assert.Contains(t, resp.Warnings[0], `additional namespace of project "foo"`)
		assert.Contains(t, resp.Warnings[1], `additional namespace of project "foo-bar"`)
	})

	t.Run("the warning comes with a project on a template as well", func(t *testing.T) {
		template := &v1alpha2.ProjectTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "team"},
			Spec: v1alpha2.ProjectTemplateSpec{
				ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{"type": "object"}},
			},
		}
		v := newValidator(t, project("foo"), template)
		created := project("foo-bar")
		created.Spec.ProjectTemplateName = "team"
		raw, err := json.Marshal(created)
		require.NoError(t, err)
		resp := v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}})
		require.True(t, resp.Allowed, resp.Result)
		require.Len(t, resp.Warnings, 1)
		assert.Contains(t, resp.Warnings[0], `additional namespace of project "foo"`)
	})

	t.Run("a name that existing projects extend is allowed, with a warning naming them", func(t *testing.T) {
		v := newValidator(t, project("foo-qux"), project("foo-bar"))
		resp := v.Handle(context.Background(), createReq(t, "foo"))
		require.True(t, resp.Allowed, resp.Result)
		require.Len(t, resp.Warnings, 1)
		assert.Contains(t, resp.Warnings[0], `named like additional namespaces of project "foo" are separate projects: "foo-bar", "foo-qux"`)
	})

	t.Run("a virtual project has no additional namespaces to be confused with", func(t *testing.T) {
		virtual := project(projectmanager.DefaultProjectName)
		virtual.Labels = map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}
		v := newValidator(t, virtual)
		resp := v.Handle(context.Background(), createReq(t, projectmanager.DefaultProjectName+"-apps"))
		assert.True(t, resp.Allowed, resp.Result)
		assert.Empty(t, resp.Warnings)
	})

	t.Run("a name a ProjectNamespace has claimed is refused", func(t *testing.T) {
		// The ProjectNamespace claims "foo-bar" the moment it is created; the namespace appears only
		// when the controller gets to it, so there is no namespace to find yet.
		claim := &v1alpha3.ProjectNamespace{
			ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "foo"},
			Spec:       v1alpha3.ProjectNamespaceSpec{Name: "bar"},
		}
		v := newValidator(t, project("foo"), claim)
		resp := v.Handle(context.Background(), createReq(t, "foo-bar"))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "the 'cache' ProjectNamespace of the 'foo' project claims a namespace with its name")
	})

	t.Run("a claim the cache has not seen yet is read from the API server", func(t *testing.T) {
		// A GitOps sync applies a ProjectNamespace and a Project under its resulting name one after
		// the other, so the claim may reach the API server before the cache of the webhook.
		claim := &v1alpha3.ProjectNamespace{
			ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "foo"},
			Spec:       v1alpha3.ProjectNamespaceSpec{Name: "bar"},
		}
		cached := newValidator(t, project("foo"))
		live := newValidator(t, project("foo"), claim)
		v := &validator{client: cached.client, reader: live.client}
		resp := v.Handle(context.Background(), createReq(t, "foo-bar"))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "the 'cache' ProjectNamespace of the 'foo' project claims a namespace with its name")
	})

	t.Run("an unrelated name is allowed beside both, without a warning", func(t *testing.T) {
		v := newValidator(t, project("foo"))
		resp := v.Handle(context.Background(), createReq(t, "foobar"))
		assert.True(t, resp.Allowed, resp.Result)
		assert.Empty(t, resp.Warnings)
	})
}

// A binding can still name the ServiceAccounts of a namespace that is gone, a deleted additional
// namespace for one. A project under that name would hand them to its creator, so the name is refused
// until the subjects are removed, and the refusal names neither the binding nor its project.
func TestHandle_NameAStoredBindingGrantsTo(t *testing.T) {
	createReq := func(t *testing.T, user, name string) admission.Request {
		t.Helper()
		raw, err := json.Marshal(&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}})
		require.NoError(t, err)
		return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: user},
			Object:    runtime.RawExtension{Raw: raw},
		}}
	}
	serviceAccount := func(namespace string) rbacv1.Subject {
		return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "deployer", Namespace: namespace}
	}
	projectBinding := func(namespace string, subject rbacv1.Subject) *v1alpha3.ProjectRoleBinding {
		return &v1alpha3.ProjectRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "leaky", Namespace: namespace},
			Spec:       v1alpha3.ProjectRoleBindingSpec{Subjects: []rbacv1.Subject{subject}},
		}
	}
	clusterBinding := &v1alpha3.ClusterProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "leaky"},
		Spec:       v1alpha3.ClusterProjectRoleBindingSpec{Subjects: []rbacv1.Subject{serviceAccount("tools-ci")}},
	}
	foo := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

	t.Run("a binding of the project the name extends", func(t *testing.T) {
		v := newValidator(t, foo, projectBinding("foo", serviceAccount("foo-extra")))
		resp := v.Handle(context.Background(), createReq(t, "bob", "foo-extra"))
		assert.False(t, resp.Allowed)
		assert.Equal(t, "The 'foo-extra' project cannot be created, a ProjectRoleBinding of another project still grants rights to "+
			"ServiceAccounts of a namespace with this name. These subjects have to be removed from that binding first", resp.Result.Message)
	})

	t.Run("a cluster-wide binding", func(t *testing.T) {
		v := newValidator(t, clusterBinding)
		resp := v.Handle(context.Background(), createReq(t, "bob", "tools-ci"))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "The 'tools-ci' project cannot be created, a ClusterProjectRoleBinding still grants rights to ServiceAccounts of a namespace with this name")
		assert.NotContains(t, resp.Result.Message, "leaky")
	})

	t.Run("the group of the service accounts of the namespace", func(t *testing.T) {
		v := newValidator(t, foo, projectBinding("foo", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:foo-extra"}))
		resp := v.Handle(context.Background(), createReq(t, "bob", "foo-extra"))
		assert.False(t, resp.Allowed)
	})

	t.Run("bindings of other namespaces leave the name free", func(t *testing.T) {
		v := newValidator(t, foo, clusterBinding, projectBinding("foo", serviceAccount("foo-queue")))
		resp := v.Handle(context.Background(), createReq(t, "bob", "foo-extra"))
		assert.True(t, resp.Allowed, resp.Result)
		require.Len(t, resp.Warnings, 1, "the name still reads like an additional namespace of foo")
	})

	// The controller adopts a namespace that exists already, so there is no name left to protect.
	t.Run("the controller adopting a namespace", func(t *testing.T) {
		v := newValidator(t, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tools-ci"}}, clusterBinding)
		resp := v.Handle(context.Background(), createReq(t, rolebindingwebhook.ControllerServiceAccount, "tools-ci"))
		assert.True(t, resp.Allowed, resp.Result)
	})
}

// The "virtual" template belongs to the platform's virtual projects. A user project on it would be
// handled as virtual by the project controller and as ordinary by everything keyed on the
// virtual-project label, and every ClusterProjectRoleBinding would then fail on its namespace.
func TestHandle_VirtualTemplateIsReserved(t *testing.T) {
	createReq := func(t *testing.T, user, name string) admission.Request {
		t.Helper()
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
		project.Spec.ProjectTemplateName = projectmanager.VirtualTemplate
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: user},
			Object:    runtime.RawExtension{Raw: raw},
		}}
	}

	t.Run("a user may not create a project on the virtual template", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(context.Background(), createReq(t, "alice", "virtual"))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "reserved for the platform's virtual projects")
	})

	t.Run("a user may not move an existing project onto the virtual template", func(t *testing.T) {
		v := newValidator(t)
		old := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}}
		old.Spec.ProjectTemplateName = "default"
		updated := old.DeepCopy()
		updated.Spec.ProjectTemplateName = projectmanager.VirtualTemplate
		resp := v.Handle(context.Background(), updateRequest(t, "alice", old, updated))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "reserved for the platform's virtual projects")
	})

	t.Run("a user may not take the platform's virtual project names either, whatever the template", func(t *testing.T) {
		v := newValidator(t)
		for _, name := range []string{projectmanager.DefaultProjectName, projectmanager.DeckhouseProjectName} {
			project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
			project.Spec.ProjectTemplateName = "default"
			raw, err := json.Marshal(project)
			require.NoError(t, err)
			resp := v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				UserInfo:  authnv1.UserInfo{Username: "alice"},
				Object:    runtime.RawExtension{Raw: raw},
			}})
			assert.False(t, resp.Allowed, name)
			assert.Contains(t, resp.Result.Message, "reserved for the platform's virtual projects", name)
		}
	})

	t.Run("the controller keeps creating the platform's virtual projects", func(t *testing.T) {
		v := newValidator(t)
		for _, name := range []string{projectmanager.DefaultProjectName, projectmanager.DeckhouseProjectName} {
			resp := v.Handle(context.Background(), createReq(t, rolebindingwebhook.ControllerServiceAccount, name))
			assert.True(t, resp.Allowed, name)
		}
	})
}

// TestHandle_ProjectLabelIsTheProjectName: projects.deckhouse.io/project on a Project names that
// project and nothing else, since grant policies select projects by it. Removing it is let through with
// a warning, since the controller puts it back.
func TestHandle_ProjectLabelIsTheProjectName(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()

	withLabel := func(value string) *v1alpha3.Project {
		p := projectWithParameters(nil)
		if value != "" {
			p.Labels = map[string]string{v1alpha3.ResourceLabelProject: value}
		}
		return p
	}

	tests := []struct {
		name     string
		user     string
		value    string
		expected bool
		warns    bool
	}{
		{name: "the project name", user: "alice", value: "foo", expected: true},
		{name: "removing the label", user: "alice", value: "", expected: true, warns: true},
		{name: "another project's name", user: "alice", value: "bar", expected: false},
		{name: "the controller is not checked", user: rolebindingwebhook.ControllerServiceAccount, value: "bar", expected: true},
		{name: "the controller may remove it", user: rolebindingwebhook.ControllerServiceAccount, value: "", expected: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := v.Handle(ctx, updateRequest(t, tt.user, withLabel("foo"), withLabel(tt.value)))
			assert.Equal(t, tt.expected, resp.Allowed, "%v", resp.Result)
			if !tt.warns {
				assert.Empty(t, resp.Warnings)
				return
			}
			require.Len(t, resp.Warnings, 1)
			assert.Contains(t, resp.Warnings[0], v1alpha3.ResourceLabelProject)
			assert.Contains(t, resp.Warnings[0], "puts it back")
		})
	}

	t.Run("a project that never had the label, before the controller stamps it", func(t *testing.T) {
		resp := v.Handle(ctx, updateRequest(t, "alice", withLabel(""), withLabel("")))
		assert.True(t, resp.Allowed, "%v", resp.Result)
		assert.Empty(t, resp.Warnings)
	})
}

// TestHandle_ProjectTemplateLabelFollowsTheTemplate: the project-template label says which template the
// project uses, and template-managed policies and template wake-ups rely on it. It keeps its value or
// takes the one of spec.projectTemplateName and cannot name another template. Removing it is let through
// with a warning, since the controller puts it back.
func TestHandle_ProjectTemplateLabelFollowsTheTemplate(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()

	project := func(template, label string) *v1alpha3.Project {
		p := projectWithParameters(nil)
		p.Spec.ProjectTemplateName = template
		if label != "" {
			p.Labels = map[string]string{v1alpha3.ResourceLabelTemplate: label}
		}
		return p
	}

	tests := []struct {
		name     string
		old      *v1alpha3.Project
		updated  *v1alpha3.Project
		expected bool
	}{
		{name: "unchanged", old: project("a", "a"), updated: project("a", "a"), expected: true},
		{name: "a template switch keeps the old label until the controller stamps it", old: project("a", "a"), updated: project("b", "a"), expected: true},
		{name: "a template switch that sets the new label", old: project("a", "a"), updated: project("b", "b"), expected: true},
		{name: "another template", old: project("a", "a"), updated: project("a", "c"), expected: false},
		{name: "removing the label", old: project("a", "a"), updated: project("a", ""), expected: true},
		{name: "setting a wrong label where there was none", old: project("a", ""), updated: project("a", "c"), expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := v.Handle(ctx, updateRequest(t, "alice", tt.old, tt.updated))
			assert.Equal(t, tt.expected, resp.Allowed, "%v", resp.Result)
		})
	}

	// A manifest that replaces the whole Project (kubectl replace, Argo CD with Replace=true) carries
	// neither label. The update goes through with one warning per label, ahead of the warnings of the
	// later checks, and the controller stamps them again.
	t.Run("a replace without the labels the controller stamped", func(t *testing.T) {
		old := project("a", "a")
		old.Labels[v1alpha3.ResourceLabelProject] = "foo"
		resp := v.Handle(ctx, updateRequest(t, "alice", old, project("a", "")))
		require.True(t, resp.Allowed, "%v", resp.Result)
		require.GreaterOrEqual(t, len(resp.Warnings), 2, "%v", resp.Warnings)
		assert.Contains(t, resp.Warnings[0], v1alpha3.ResourceLabelProject+" label")
		assert.Contains(t, resp.Warnings[1], v1alpha3.ResourceLabelTemplate+" label")
	})
}

// TestHandle_ModuleOwnedNamespaceLabels: the built-in templates take namespaceMetadata.labels from the
// namespace.labels parameter, so only the Project webhook sees what a project sets there, and a label
// the module owns is refused instead of being dropped by the renderer. A project that already carries
// one stays editable: only a label set anew is refused, a kept one is named in a warning. The
// controller and Deckhouse are not checked.
func TestHandle_ModuleOwnedNamespaceLabels(t *testing.T) {
	template := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tmpl"},
		Spec: v1alpha2.ProjectTemplateSpec{
			NamespaceMetadata: &v1alpha2.NamespaceMetadata{Labels: v1alpha2.FromParamRef[map[string]string]("namespace.labels")},
			ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
				"type": "object", "properties": map[string]any{"namespace": map[string]any{"type": "object", "properties": map[string]any{
					"labels": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				}}},
			}},
		},
	}
	v := newValidator(t, template)
	ctx := context.Background()

	withLabels := func(labels map[string]any) *v1alpha3.Project {
		project := projectWithParameters(map[string]any{"namespace": map[string]any{"labels": labels}})
		project.Spec.ProjectTemplateName = "tmpl"
		project.Labels = map[string]string{v1alpha3.ResourceLabelTemplate: "tmpl"}
		return project
	}
	privileged := map[string]any{"security.deckhouse.io/pod-policy": "privileged"}
	privilegedAndTeam := map[string]any{"security.deckhouse.io/pod-policy": "privileged", "team": "a"}

	t.Run("create", func(t *testing.T) {
		raw, err := json.Marshal(withLabels(privileged))
		require.NoError(t, err)
		resp := v.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}})
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "the parameter 'namespace.labels' sets namespace labels the module owns")
		assert.Contains(t, resp.Result.Message, "security.deckhouse.io/pod-policy (set it through spec.podSecurityStandard)")
	})

	// The warning of a kept label joins the list of the earlier checks: an edit that also drops the
	// project label the controller stamped gets both warnings, the one about the dropped project label
	// first.
	t.Run("an edit that keeps a label the project had and removes the project label", func(t *testing.T) {
		old := withLabels(privileged)
		old.Labels[v1alpha3.ResourceLabelProject] = "foo"
		resp := v.Handle(ctx, updateRequest(t, "alice", old, withLabels(privilegedAndTeam)))
		require.True(t, resp.Allowed, "%v", resp.Result)
		assert.Equal(t, []string{
			`the projects.deckhouse.io/project label of the "foo" project is set by the multitenancy-manager controller, which puts it back`,
			"the parameter 'namespace.labels' of the 'foo' project sets namespace labels the module owns, and the project does not apply them: " +
				"security.deckhouse.io/pod-policy (set it through spec.podSecurityStandard); remove them from the parameter",
		}, resp.Warnings)
	})

	tests := []struct {
		name     string
		user     string
		old      *v1alpha3.Project
		updated  *v1alpha3.Project
		allowed  bool
		mentions string
		warns    string
	}{
		{
			name:     "a pod policy label set anew",
			user:     "alice",
			old:      withLabels(map[string]any{"team": "a"}),
			updated:  withLabels(privilegedAndTeam),
			mentions: "security.deckhouse.io/pod-policy (set it through spec.podSecurityStandard)",
		},
		{
			name:     "a label of the controller set anew",
			user:     "alice",
			old:      withLabels(nil),
			updated:  withLabels(map[string]any{"projects.deckhouse.io/project": "other"}),
			mentions: "projects.deckhouse.io/project (the controller sets it)",
		},
		{
			name:     "a new value of a label the project had",
			user:     "alice",
			old:      withLabels(privileged),
			updated:  withLabels(map[string]any{"security.deckhouse.io/pod-policy": "baseline"}),
			mentions: "security.deckhouse.io/pod-policy",
		},
		{
			name:     "a label kept on a template switch is set anew",
			user:     "alice",
			old:      func() *v1alpha3.Project { p := withLabels(privileged); p.Spec.ProjectTemplateName = "other"; return p }(),
			updated:  withLabels(privileged),
			mentions: "security.deckhouse.io/pod-policy",
		},
		{
			name:    "an ordinary label",
			user:    "alice",
			old:     withLabels(nil),
			updated: withLabels(map[string]any{"team": "a"}),
			allowed: true,
		},
		{
			name:    "an edit that keeps a label the project had",
			user:    "alice",
			old:     withLabels(privileged),
			updated: withLabels(privilegedAndTeam),
			allowed: true,
			warns:   "security.deckhouse.io/pod-policy (set it through spec.podSecurityStandard)",
		},
		{
			name:    "removing a label the project had",
			user:    "alice",
			old:     withLabels(privilegedAndTeam),
			updated: withLabels(map[string]any{"team": "a"}),
			allowed: true,
		},
		{
			name: "the controller stamps a project that carries one",
			user: rolebindingwebhook.ControllerServiceAccount,
			old: func() *v1alpha3.Project {
				p := withLabels(privileged)
				p.Annotations = map[string]string{v1alpha3.ProjectAnnotationRequireSync: "true"}
				return p
			}(),
			updated: func() *v1alpha3.Project {
				p := withLabels(privileged)
				p.Labels[v1alpha3.ResourceLabelProject] = p.Name
				p.Finalizers = []string{v1alpha3.ProjectFinalizer}
				return p
			}(),
			allowed: true,
		},
		{
			name: "the controller removes the finalizer of a deleted project that carries one",
			user: rolebindingwebhook.ControllerServiceAccount,
			old: func() *v1alpha3.Project {
				p := withLabels(privileged)
				p.Finalizers = []string{v1alpha3.ProjectFinalizer}
				p.DeletionTimestamp = &metav1.Time{}
				return p
			}(),
			updated: func() *v1alpha3.Project {
				p := withLabels(privileged)
				p.DeletionTimestamp = &metav1.Time{}
				return p
			}(),
			allowed: true,
		},
		{
			name:    "Deckhouse is not checked either",
			user:    rolebindingwebhook.DeckhouseServiceAccount,
			old:     withLabels(nil),
			updated: withLabels(privileged),
			allowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := v.Handle(ctx, updateRequest(t, tt.user, tt.old, tt.updated))
			require.Equal(t, tt.allowed, resp.Allowed, resp.Result.Message)
			if tt.mentions != "" {
				assert.Contains(t, resp.Result.Message, "the parameter 'namespace.labels' sets namespace labels the module owns")
				assert.Contains(t, resp.Result.Message, tt.mentions)
			}
			if tt.warns == "" {
				assert.Empty(t, resp.Warnings)
				return
			}
			require.Len(t, resp.Warnings, 1)
			assert.Contains(t, resp.Warnings[0], "the project does not apply them")
			assert.Contains(t, resp.Warnings[0], tt.warns)
		})
	}
}

// The webhook renders a resourcesTemplate before it lets a project on: a text that does not render
// and a parameter that becomes manifest structure are refused at admission, and an object put into a
// namespace outside the project passes with a warning, since the post-renderer moves it to the main
// namespace.
func TestHandle_HelmRender(t *testing.T) {
	helmTemplate := func(name, resources string) *v1alpha2.ProjectTemplate {
		return &v1alpha2.ProjectTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1alpha2.ProjectTemplateSpec{
				ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
					"type": "object", "properties": map[string]any{"owner": map[string]any{"type": "string"}},
				}},
				ResourcesTemplate: resources,
			},
		}
	}
	v := newValidator(t,
		helmTemplate("unquoted", "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: owner\ndata:\n  owner: {{ .parameters.owner }}\n"),
		helmTemplate("broken", "{{ .parameters.owner"),
		helmTemplate("elsewhere", "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: owner\n  namespace: kube-system\n"),
	)

	create := func(t *testing.T, template, owner string) admission.Response {
		t.Helper()
		project := projectWithParameters(map[string]any{"owner": owner})
		project.Name = "team"
		project.Spec.ProjectTemplateName = template
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}})
	}

	t.Run("an ordinary parameter passes", func(t *testing.T) {
		resp := create(t, "unquoted", "alice")
		assert.True(t, resp.Allowed, resp.Result)
		assert.Empty(t, resp.Warnings)
	})

	t.Run("a parameter that becomes an object is refused", func(t *testing.T) {
		resp := create(t, "unquoted", "alice\n---\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRoleBinding\nmetadata:\n  name: escalation\n")
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, helm.ErrParameterInjection.Error())
	})

	t.Run("a text that does not render is refused", func(t *testing.T) {
		resp := create(t, "broken", "alice")
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "render chart")
	})

	t.Run("an object in a foreign namespace passes with a warning", func(t *testing.T) {
		resp := create(t, "elsewhere", "alice")
		assert.True(t, resp.Allowed, resp.Result)
		assert.Contains(t, resp.Warnings, helm.ErrNamespaceOverride.Error())
	})
}

// Only a resourcesTemplate is rehearsed through Helm. A chart that fails whatever it renders tells
// the two apart: a project on a structured template passes, one on a Helm text is refused.
func TestHandle_HelmRenderOnlyForAHelmText(t *testing.T) {
	chart := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(chart, "rehearsed.yaml"), []byte(`{{ fail "rehearsed" }}`), 0o600))
	helmClient, err := helm.NewRenderOnly(chart, logr.Discard())
	require.NoError(t, err)

	schema := v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{"type": "object"}}
	structured := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "structured"},
		Spec: v1alpha2.ProjectTemplateSpec{
			ParametersSchema:    schema,
			PodSecurityStandard: v1alpha2.LiteralParam(v1alpha2.PodSecurityStandardBaseline),
		},
	}
	helmText := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "helm"},
		Spec: v1alpha2.ProjectTemplateSpec{
			ParametersSchema:  schema,
			ResourcesTemplate: "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n",
		},
	}
	v := newValidator(t, structured, helmText)
	v.helmClient = helmClient

	create := func(t *testing.T, template string) admission.Response {
		t.Helper()
		project := projectWithParameters(nil)
		project.Name = "team"
		project.Spec.ProjectTemplateName = template
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}})
	}

	resp := create(t, "structured")
	assert.True(t, resp.Allowed, resp.Result)

	resp = create(t, "helm")
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "rehearsed")
}

// The request is read as JSON, so an administrator name keeps an unescaped U+0085 that a YAML
// decoder would fold into a space, and the check sees it. A name with a line break or a C1 control is
// refused unless the update keeps it.
func TestHandle_ControlCharacterInAnAdministratorName(t *testing.T) {
	v := newValidator(t)

	for _, name := range []string{"alice\u0085x", "alice\u2028x", "alice\u2029x", "alice\u0080x", "alice\u009bx"} {
		raw, err := json.Marshal(&v1alpha3.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "team"},
			Spec:       v1alpha3.ProjectSpec{Administrators: []v1alpha3.Administrator{{Kind: "User", Name: name}}},
		})
		require.NoError(t, err)

		resp := v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}})
		require.False(t, resp.Allowed, "%q", name)
		assert.Contains(t, resp.Result.Message, "must not contain control characters or line breaks")

		// an update that keeps the name passes with a warning
		resp = v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
			OldObject: runtime.RawExtension{Raw: raw},
		}})
		require.True(t, resp.Allowed, "%q: %v", name, resp.Result)
		assert.NotEmpty(t, resp.Warnings, "%q", name)
	}
}

// The v1alpha2 -> v1alpha3 conversion leaves administrators or resourceQuota in spec.parameters when it
// cannot give them back as they were, and the built-in templates declare neither any more, so there
// the two grant no access and set no quota. A project that holds them, as e2e-bidef-zero and
// e2e-bidef-noadm of the upgrade e2e test do on the built-in default template, can be written as it
// is, edited and moved to another built-in template that keeps the value, with a warning, and so can
// a project moved there from a template that declares the two. A value that grants or limits nothing
// passes on create too, with a warning. Any other new or changed value is refused and pointed to the
// standard field, and a request through v1alpha2, which has only the parameters, is told the shape
// the conversion lifts. Another parameter the schema does not declare is still refused, and a
// template that declares the two still checks them.
func TestHandle_WhatTheConversionLeavesInTheParameters(t *testing.T) {
	builtin := func(file string) *v1alpha2.ProjectTemplate {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", file))
		require.NoError(t, err)
		template := new(v1alpha2.ProjectTemplate)
		require.NoError(t, yaml.Unmarshal(raw, template))
		return template
	}
	const declaring = `
type: object
properties:
  administrators:
    type: array
    minItems: 1
    items:
      type: object
      properties:
        subject: {type: string, enum: [User, Group]}
        name: {type: string}
  resourceQuota:
    type: object
    properties:
      requests: {type: object, properties: {cpu: {type: string}, memory: {type: string}}}
`
	openAPI := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(declaring), &openAPI))
	declares := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "declares"},
		Spec: v1alpha2.ProjectTemplateSpec{
			ParametersSchema:    v1alpha2.ParametersSchema{OpenAPIV3Schema: openAPI},
			PodSecurityStandard: v1alpha2.LiteralParam(v1alpha2.PodSecurityStandardBaseline),
		},
	}
	v := newValidator(t, builtin("default.yaml"), builtin("secure.yaml"), declares)
	ctx := context.Background()

	onDefault := func(parameters map[string]any) *v1alpha3.Project {
		return &v1alpha3.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "team"},
			Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "default", Parameters: parameters},
		}
	}
	// through names the version a request is written in, as the API server reports it to a webhook
	// with matchPolicy: Equivalent
	through := func(req admission.Request, version string) admission.Request {
		req.RequestKind = &metav1.GroupVersionKind{Group: "deckhouse.io", Version: version, Kind: "Project"}
		return req
	}
	create := func(t *testing.T, project *v1alpha3.Project, version string) admission.Response {
		t.Helper()
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return v.Handle(ctx, through(admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: "alice"},
			Object:    runtime.RawExtension{Raw: raw},
		}}, version))
	}
	update := func(t *testing.T, user string, old, updated *v1alpha3.Project, version string) admission.Response {
		t.Helper()
		return v.Handle(ctx, through(updateRequest(t, user, old, updated), version))
	}
	const shapeOfAdministrators = "which a deckhouse.io/v1alpha2 manifest sets through this parameter when it is a non-empty list whose items hold only a subject, User or Group, and a name that is not empty and has no ASCII control character"
	const shapeOfQuota = `which a deckhouse.io/v1alpha2 manifest sets through this parameter when it is a non-empty object whose top-level keys do not start with "requests." or "limits.", whose requests and limits, if they are objects, are not empty, and whose other values and the values under requests and limits are quantities such as 2, 500m or 1Gi other than the number 0, which set no limit; leave such a limit out to keep it unset, since the string "0" is a hard limit of zero`

	t.Run("a value that grants or limits nothing passes with a warning", func(t *testing.T) {
		tests := []struct {
			name      string
			parameter string
			value     any
		}{
			{name: "an empty administrators list", parameter: "administrators", value: []any{}},
			{name: "null administrators", parameter: "administrators", value: nil},
			{name: "an empty quota", parameter: "resourceQuota", value: map[string]any{}},
			{name: "a null quota", parameter: "resourceQuota", value: nil},
			{
				name:      "a quota of the number 0, nulls and empty requests",
				parameter: "resourceQuota",
				value: map[string]any{
					"pods":     float64(0),
					"requests": map[string]any{},
					"limits":   map[string]any{"cpu": float64(0), "memory": nil},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := create(t, onDefault(map[string]any{tt.parameter: tt.value}), "v1alpha3")
				require.True(t, resp.Allowed, resp.Result)
				require.Len(t, resp.Warnings, 1)
				assert.Equal(t, "the parameter '"+tt.parameter+"' of the 'team' project has no effect on the 'default' project template, whose parametersSchema does not declare it; remove it from the parameters",
					resp.Warnings[0])
			})
		}
	})

	t.Run("a new value that would grant or limit something is refused", func(t *testing.T) {
		tests := []struct {
			name      string
			version   string
			parameter string
			value     any
			expected  string
		}{
			{
				// a later write through v1alpha2 would lift it into spec.administrators
				name:      "administrators a v1alpha3 client puts into the parameters",
				version:   "v1alpha3",
				parameter: "administrators",
				value:     []any{map[string]any{"subject": "User", "name": "alice"}},
				expected:  "grants no access; set spec.administrators instead",
			},
			{
				name:      "administrators with an access level written through v1alpha2",
				version:   "v1alpha2",
				parameter: "administrators",
				value:     []any{map[string]any{"subject": "User", "name": "alice", "accessLevel": "Admin"}},
				expected:  "grants no access; set spec.administrators instead, " + shapeOfAdministrators,
			},
			{
				// the quota 1.77 limited the memory of
				name:      "a quota with the number 0 written through v1alpha2",
				version:   "v1alpha2",
				parameter: "resourceQuota",
				value:     map[string]any{"requests": map[string]any{"cpu": float64(0), "memory": "2Gi"}},
				expected:  "sets no quota; set spec.quota instead, " + shapeOfQuota,
			},
			{
				// a later write through v1alpha2 would lift it into spec.quota as a limit of zero
				name:      "a quota of the string 0 a v1alpha3 client puts into the parameters",
				version:   "v1alpha3",
				parameter: "resourceQuota",
				value:     map[string]any{"requests": map[string]any{"cpu": "0"}},
				expected:  "sets no quota; set spec.quota instead",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := create(t, onDefault(map[string]any{tt.parameter: tt.value}), tt.version)
				require.False(t, resp.Allowed)
				assert.Equal(t,
					"The project 'team' is invalid: the parameter '"+tt.parameter+"' has no effect on the 'default' project template, whose parametersSchema does not declare it, and "+tt.expected,
					resp.Result.Message)
			})
		}
	})

	t.Run("a template that declares the two takes them without a warning", func(t *testing.T) {
		project := onDefault(map[string]any{
			"administrators": []any{map[string]any{"subject": "User", "name": "alice"}},
			"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}},
		})
		project.Spec.ProjectTemplateName = "declares"
		resp := create(t, project, "v1alpha3")
		assert.True(t, resp.Allowed, resp.Result)
		assert.Empty(t, resp.Warnings)
	})

	stored := []struct {
		name    string
		project *v1alpha3.Project
		// parameter is the one the conversion left in the parameters, and changed another value of it
		parameter string
		changed   any
		// kept is the end of the warning for an update that keeps the value, after the template
		kept string
		// keptThroughV1alpha2 is that end for an update through v1alpha2
		keptThroughV1alpha2 string
		// refused is a part of the denial on a move to the template that declares the two
		refused string
	}{
		{
			name: "e2e-bidef-zero, a quota with the number 0",
			project: func() *v1alpha3.Project {
				project := onDefault(map[string]any{
					"resourceQuota": map[string]any{"requests": map[string]any{"cpu": float64(0), "memory": "2Gi"}},
				})
				project.Spec.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}}
				return project
			}(),
			parameter:           "resourceQuota",
			changed:             map[string]any{"requests": map[string]any{"cpu": float64(0), "memory": "3Gi"}},
			kept:                ", and sets no quota; it is accepted because it did not change, and spec.quota is the place for a quota",
			keptThroughV1alpha2: ", and sets no quota; it is accepted because it did not change, and spec.quota is the place for a quota, " + shapeOfQuota,
			refused:             "resourceQuota.requests.cpu in body must be of type string",
		},
		{
			name: "e2e-bidef-noadm, an empty administrators list",
			project: func() *v1alpha3.Project {
				project := onDefault(map[string]any{"administrators": []any{}})
				project.Spec.Quota = corev1.ResourceList{"requests.cpu": resource.MustParse("1")}
				return project
			}(),
			parameter:           "administrators",
			changed:             []any{map[string]any{"subject": "User", "name": "bob"}},
			kept:                "; remove it from the parameters",
			keptThroughV1alpha2: "; remove it from the parameters",
			refused:             "administrators in body should have at least 1 items",
		},
		{
			name: "administrators with an access level",
			project: onDefault(map[string]any{
				"administrators": []any{map[string]any{"subject": "User", "name": "alice", "accessLevel": "Admin"}},
			}),
			parameter:           "administrators",
			changed:             []any{map[string]any{"subject": "User", "name": "bob", "accessLevel": "Admin"}},
			kept:                ", and grants no access; it is accepted because it did not change, and spec.administrators is the place for administrators",
			keptThroughV1alpha2: ", and grants no access; it is accepted because it did not change, and spec.administrators is the place for administrators, " + shapeOfAdministrators,
			refused:             "administrators.accessLevel in body is a forbidden property",
		},
	}
	for _, tt := range stored {
		t.Run(tt.name, func(t *testing.T) {
			warned := func(t *testing.T, resp admission.Response, template, end string) {
				t.Helper()
				require.True(t, resp.Allowed, resp.Result)
				require.Len(t, resp.Warnings, 1)
				assert.Equal(t, "the parameter '"+tt.parameter+"' of the 'team' project has no effect on the '"+template+"' project template, whose parametersSchema does not declare it"+end,
					resp.Warnings[0])
			}

			warned(t, update(t, "alice", tt.project, tt.project.DeepCopy(), "v1alpha3"), "default", tt.kept)

			edited := tt.project.DeepCopy()
			edited.Spec.Description = "edited"
			edited.Spec.Parameters["networkPolicy"] = "NotRestricted"
			warned(t, update(t, "alice", tt.project, edited, "v1alpha2"), "default", tt.keptThroughV1alpha2)

			moved := tt.project.DeepCopy()
			moved.Spec.ProjectTemplateName = "secure"
			warned(t, update(t, "alice", tt.project, moved, "v1alpha3"), "secure", tt.kept)

			// the value is kept whatever template the stored project is on, one that declares the two
			// included
			fromDeclaring := tt.project.DeepCopy()
			fromDeclaring.Spec.ProjectTemplateName = "declares"
			warned(t, update(t, "alice", fromDeclaring, tt.project.DeepCopy(), "v1alpha3"), "default", tt.kept)

			changed := tt.project.DeepCopy()
			changed.Spec.Parameters[tt.parameter] = tt.changed
			resp := update(t, "alice", tt.project, changed, "v1alpha3")
			assert.False(t, resp.Allowed, "changed")
			assert.Contains(t, resp.Result.Message, "the parameter '"+tt.parameter+"' has no effect on the 'default' project template")

			// the controller, which the webhook configuration leaves out anyway, is not checked
			resp = update(t, rolebindingwebhook.ControllerServiceAccount, tt.project, changed, "v1alpha3")
			assert.True(t, resp.Allowed, "changed by the controller: %v", resp.Result)
			assert.Empty(t, resp.Warnings)

			undeclared := tt.project.DeepCopy()
			undeclared.Spec.Parameters["owner"] = "alice"
			resp = update(t, "alice", tt.project, undeclared, "v1alpha3")
			assert.False(t, resp.Allowed, "another undeclared parameter")
			assert.Contains(t, resp.Result.Message, "owner in body is a forbidden property")

			moved.Spec.ProjectTemplateName = "declares"
			resp = update(t, "alice", tt.project, moved, "v1alpha3")
			assert.False(t, resp.Allowed, "moved to a template that declares the two")
			assert.Contains(t, resp.Result.Message, tt.refused)
		})
	}
}
