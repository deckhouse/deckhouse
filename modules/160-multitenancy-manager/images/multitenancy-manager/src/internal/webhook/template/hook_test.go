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

package template

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha2.AddToScheme, v1alpha3.AddToScheme, grantsv1alpha1.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c, reader: c}
}

func libraryPolicy(name string) *grantsv1alpha1.ClusterResourceGrantPolicy {
	return &grantsv1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       grantsv1alpha1.ClusterResourceGrantPolicySpec{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}},
	}
}

func boundPolicy(name string) *grantsv1alpha1.ClusterResourceGrantPolicy {
	p := libraryPolicy(name)
	p.Spec.ProjectSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"example": "true"}}
	return p
}

func createRequest(t *testing.T, tmpl *v1alpha2.ProjectTemplate) admission.Request {
	t.Helper()
	tmpl.TypeMeta = metav1.TypeMeta{APIVersion: v1alpha2.SchemeGroupVersion.String(), Kind: v1alpha2.ProjectTemplateKind}
	raw, err := json.Marshal(tmpl)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: raw},
	}}
}

func structuredTemplate(name string, grantPolicies ...string) *v1alpha2.ProjectTemplate {
	return &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha2.ProjectTemplateSpec{
			PodSecurityStandard: v1alpha2.LiteralParam(v1alpha2.PodSecurityStandardBaseline),
			GrantPolicies:       grantPolicies,
		},
	}
}

func TestHandle_GrantPoliciesValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("reference to an existing library policy is allowed", func(t *testing.T) {
		v := newValidator(t, libraryPolicy("lib"))
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl", "lib")))
		assert.True(t, resp.Allowed)
	})

	t.Run("reference to a missing policy is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl", "absent")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "does not exist")
	})

	t.Run("reference to a policy with a projectSelector is denied", func(t *testing.T) {
		v := newValidator(t, boundPolicy("bound"))
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl", "bound")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "projectSelector")
	})

	t.Run("template without grantPolicies is allowed", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl")))
		assert.True(t, resp.Allowed)
	})
}

func TestHandle_ManagedGrantNameCollision(t *testing.T) {
	ctx := context.Background()

	t.Run("a grant policy named 'inline' is reserved", func(t *testing.T) {
		// the reference resolves (library policy exists) but its name collides with the inline slot
		v := newValidator(t, libraryPolicy("inline"))
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl", "inline")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "reserved for the inline managed policy slot")
	})

	t.Run("two templates whose managed names collide are rejected", func(t *testing.T) {
		// template "a" + policy "b-c" and template "a-b" + policy "c" both map to "template-a-b-c"
		existing := structuredTemplate("a-b", "c")
		v := newValidator(t, libraryPolicy("b-c"), existing)
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("a", "b-c")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "collides with the 'a-b' project template")
	})

	t.Run("distinct managed names across templates are allowed", func(t *testing.T) {
		existing := structuredTemplate("other", "lib-other")
		v := newValidator(t, libraryPolicy("lib"), libraryPolicy("lib-other"), existing)
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl", "lib")))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})
}

func TestHandle_FromParamValidation(t *testing.T) {
	ctx := context.Background()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"podSecurityProfile": map[string]any{"type": "string"},
			"namespace": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"labels": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				},
			},
		},
	}

	withRefs := func(podSec, nsLabels string) *v1alpha2.ProjectTemplate {
		tmpl := &v1alpha2.ProjectTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "tmpl"},
			Spec: v1alpha2.ProjectTemplateSpec{
				PodSecurityStandard: v1alpha2.FromParamRef[string](podSec),
				NamespaceMetadata:   &v1alpha2.NamespaceMetadata{Labels: v1alpha2.FromParamRef[map[string]string](nsLabels)},
				ParametersSchema:    v1alpha2.ParametersSchema{OpenAPIV3Schema: schema},
			},
		}
		return tmpl
	}

	t.Run("references to declared parameters are allowed", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, withRefs("podSecurityProfile", "namespace.labels")))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("reference to an undeclared top-level parameter is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, withRefs("nope", "namespace.labels")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "podSecurityStandard")
		assert.Contains(t, resp.Result.Message, "not declared in spec.parametersSchema.properties")
	})

	t.Run("reference to an undeclared nested path is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, withRefs("podSecurityProfile", "namespace.missing")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "namespaceMetadata.labels")
		assert.Contains(t, resp.Result.Message, "property 'namespace'")
	})
}

// TestHandle_FromParamTypeValidation pins the fromParam type-compatibility check: a field bound to a
// parameter whose declared schema type cannot satisfy it must be denied at admission, not at project
// render time. A parameter without a declared type is accepted (shape is unknown, render decides).
func TestHandle_FromParamTypeValidation(t *testing.T) {
	ctx := context.Background()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"strParam":     map[string]any{"type": "string"},
			"boolParam":    map[string]any{"type": "boolean"},
			"untypedParam": map[string]any{},
		},
	}

	tmpl := func(mutate func(*v1alpha2.ProjectTemplateSpec)) *v1alpha2.ProjectTemplate {
		out := &v1alpha2.ProjectTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "tmpl"},
			Spec:       v1alpha2.ProjectTemplateSpec{ParametersSchema: v1alpha2.ParametersSchema{OpenAPIV3Schema: schema}},
		}
		mutate(&out.Spec)
		return out
	}

	t.Run("boolean field bound to a string parameter is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, tmpl(func(s *v1alpha2.ProjectTemplateSpec) {
			s.RuntimeAudit = &v1alpha2.RuntimeAuditSpec{Enabled: v1alpha2.FromParamRef[bool]("strParam")}
		})))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "runtimeAudit.enabled")
		assert.Contains(t, resp.Result.Message, "requires type 'boolean'")
	})

	t.Run("string field bound to a boolean parameter is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, tmpl(func(s *v1alpha2.ProjectTemplateSpec) {
			s.PodSecurityStandard = v1alpha2.FromParamRef[string]("boolParam")
		})))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "podSecurityStandard")
		assert.Contains(t, resp.Result.Message, "requires type 'string'")
	})

	t.Run("object field bound to a string parameter is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, tmpl(func(s *v1alpha2.ProjectTemplateSpec) {
			s.NodeSelector = v1alpha2.FromParamRef[map[string]string]("strParam")
		})))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "nodeSelector")
		assert.Contains(t, resp.Result.Message, "requires type 'object'")
	})

	t.Run("matching types are allowed", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, tmpl(func(s *v1alpha2.ProjectTemplateSpec) {
			s.PodSecurityStandard = v1alpha2.FromParamRef[string]("strParam")
			s.RuntimeAudit = &v1alpha2.RuntimeAuditSpec{Enabled: v1alpha2.FromParamRef[bool]("boolParam")}
		})))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("a parameter without a declared type is allowed for any field", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, tmpl(func(s *v1alpha2.ProjectTemplateSpec) {
			s.RuntimeAudit = &v1alpha2.RuntimeAuditSpec{Enabled: v1alpha2.FromParamRef[bool]("untypedParam")}
		})))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})
}

func TestHandle_DeleteInUseTemplate(t *testing.T) {
	ctx := context.Background()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: map[string]string{v1alpha3.ResourceLabelTemplate: "tmpl"}},
	}
	v := newValidator(t, project)

	old, err := json.Marshal(structuredTemplate("tmpl"))
	require.NoError(t, err)
	resp := v.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		OldObject: runtime.RawExtension{Raw: old},
	}})
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "cannot be deleted")
}
