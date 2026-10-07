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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	grantsv1alpha1 "controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha2.AddToScheme, v1alpha3.AddToScheme, grantsv1alpha1.AddToScheme, grantsv1alpha2.AddToScheme, corev1.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c, reader: c}
}

func libraryPolicy(name string) *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       grantsv1alpha2.ClusterResourceGrantPolicySpec{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}},
	}
}

func boundPolicy(name string) *grantsv1alpha2.ClusterResourceGrantPolicy {
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
		Operation:   admissionv1.Create,
		Kind:        v1alpha2Kind,
		RequestKind: v1alpha2Kind.DeepCopy(),
		Object:      runtime.RawExtension{Raw: raw},
	}}
}

// v1alpha2Kind is the kind the webhook gets every request as, and the kind of a request a client
// makes through v1alpha2. A test of a request through v1alpha1 sets RequestKind to v1alpha1Kind.
var (
	v1alpha2Kind = metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha2", Kind: v1alpha2.ProjectTemplateKind}
	v1alpha1Kind = metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: v1alpha2.ProjectTemplateKind}
)

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

	t.Run("reference to a policy with only a namespace selector is rejected", func(t *testing.T) {
		policy := libraryPolicy("by-namespace")
		policy.Spec.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"example": "true"}}
		v := newValidator(t, policy)
		resp := v.Handle(ctx, createRequest(t, structuredTemplate("tmpl", "by-namespace")))
		require.False(t, resp.Allowed)
		require.Contains(t, resp.Result.Message, "namespaceSelector")
	})

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

	t.Run("boolean field bound to a string parameter next to additionalProperties is denied", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, tmpl(func(s *v1alpha2.ProjectTemplateSpec) {
			s.ParametersSchema.OpenAPIV3Schema = map[string]any{
				"type":                 "object",
				"properties":           schema["properties"],
				"additionalProperties": true,
			}
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

func updateRequest(t *testing.T, oldTmpl, newTmpl *v1alpha2.ProjectTemplate) admission.Request {
	t.Helper()
	for _, tmpl := range []*v1alpha2.ProjectTemplate{oldTmpl, newTmpl} {
		tmpl.TypeMeta = metav1.TypeMeta{APIVersion: v1alpha2.SchemeGroupVersion.String(), Kind: v1alpha2.ProjectTemplateKind}
	}
	oldRaw, err := json.Marshal(oldTmpl)
	require.NoError(t, err)
	newRaw, err := json.Marshal(newTmpl)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation:   admissionv1.Update,
		Kind:        v1alpha2Kind,
		RequestKind: v1alpha2Kind.DeepCopy(),
		Object:      runtime.RawExtension{Raw: newRaw},
		OldObject:   runtime.RawExtension{Raw: oldRaw},
	}}
}

// TestHandle_LiteralValidation pins the fixed-set fields: a literal outside the set is refused with
// the set spelled out, a reference is left to the parameters-schema check, and the built-in values
// pass. A typo here used to be accepted and rendered into nothing.
func TestHandle_LiteralValidation(t *testing.T) {
	ctx := context.Background()
	withLiterals := func(pss, mode string) *v1alpha2.ProjectTemplate {
		tmpl := &v1alpha2.ProjectTemplate{ObjectMeta: metav1.ObjectMeta{Name: "tmpl"}}
		if pss != "" {
			tmpl.Spec.PodSecurityStandard = v1alpha2.LiteralParam(pss)
		}
		if mode != "" {
			tmpl.Spec.NetworkPolicy = &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(mode)}
		}
		return tmpl
	}

	t.Run("a typo in podSecurityStandard is denied with the accepted values", func(t *testing.T) {
		resp := newValidator(t).Handle(ctx, createRequest(t, withLiterals("Isolted", "")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "podSecurityStandard")
		assert.Contains(t, resp.Result.Message, "Privileged, Baseline, Restricted")
		assert.Contains(t, resp.Result.Message, "'Isolted'")
	})

	t.Run("a lower-case network policy mode is denied with the accepted values", func(t *testing.T) {
		resp := newValidator(t).Handle(ctx, createRequest(t, withLiterals("Baseline", "isolated")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "networkPolicy.mode")
		assert.Contains(t, resp.Result.Message, "Isolated, NotRestricted")
	})

	t.Run("the accepted literals pass", func(t *testing.T) {
		for _, pss := range []string{"Privileged", "Baseline", "Restricted"} {
			for _, mode := range []string{"Isolated", "NotRestricted"} {
				resp := newValidator(t).Handle(ctx, createRequest(t, withLiterals(pss, mode)))
				assert.True(t, resp.Allowed, "%s/%s: %s", pss, mode, resp.Result.Message)
			}
		}
	})

	t.Run("a reference is not a literal and is checked against the schema instead", func(t *testing.T) {
		tmpl := withLiterals("", "")
		tmpl.Spec.PodSecurityStandard = v1alpha2.FromParamRef[string]("profile")
		tmpl.Spec.ParametersSchema = v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
			"type": "object", "properties": map[string]any{"profile": map[string]any{"type": "string"}},
		}}
		resp := newValidator(t).Handle(ctx, createRequest(t, tmpl))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})
}

// TestHandle_ModuleOwnedNamespaceLabels: a literal namespaceMetadata.labels may not set a label the
// module owns; the renderer would drop it, and the template would say something its namespaces lack.
// A template that already carries one stays editable: only a label set anew is refused, a kept one
// is named in a warning.
func TestHandle_ModuleOwnedNamespaceLabels(t *testing.T) {
	ctx := context.Background()
	withLabels := func(labels v1alpha2.Param[map[string]string]) *v1alpha2.ProjectTemplate {
		tmpl := &v1alpha2.ProjectTemplate{ObjectMeta: metav1.ObjectMeta{Name: "tmpl"}}
		tmpl.Spec.NamespaceMetadata = &v1alpha2.NamespaceMetadata{Labels: labels}
		tmpl.Spec.ParametersSchema = v1alpha2.ParametersSchema{OpenAPIV3Schema: map[string]any{
			"type": "object", "properties": map[string]any{"namespace": map[string]any{"type": "object", "properties": map[string]any{
				"labels": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			}}},
		}}
		return tmpl
	}

	tests := []struct {
		name     string
		labels   v1alpha2.Param[map[string]string]
		allowed  bool
		mentions string
	}{
		{name: "a pod policy label points to the field", labels: v1alpha2.LiteralParam(map[string]string{"security.deckhouse.io/pod-policy": "privileged"}), mentions: "set it through spec.podSecurityStandard"},
		{name: "a label of the controller", labels: v1alpha2.LiteralParam(map[string]string{"projects.deckhouse.io/foo": "x"}), mentions: "projects.deckhouse.io/foo (the controller sets it)"},
		{name: "an ordinary label", labels: v1alpha2.LiteralParam(map[string]string{"team": "a"}), allowed: true},
		{name: "a reference is checked per project", labels: v1alpha2.FromParamRef[map[string]string]("namespace.labels"), allowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := newValidator(t).Handle(ctx, createRequest(t, withLabels(tt.labels)))
			require.Equal(t, tt.allowed, resp.Allowed, resp.Result.Message)
			if tt.mentions != "" {
				assert.Contains(t, resp.Result.Message, tt.mentions)
			}
		})
	}

	literal := func(labels map[string]string) v1alpha2.Param[map[string]string] { return v1alpha2.LiteralParam(labels) }
	privileged := map[string]string{"security.deckhouse.io/pod-policy": "privileged"}
	privilegedAndTeam := map[string]string{"security.deckhouse.io/pod-policy": "privileged", "team": "a"}
	updates := []struct {
		name     string
		old      v1alpha2.Param[map[string]string]
		updated  v1alpha2.Param[map[string]string]
		allowed  bool
		mentions string
		warns    string
	}{
		{name: "a label set anew", old: literal(map[string]string{"team": "a"}), updated: literal(privilegedAndTeam), mentions: "set it through spec.podSecurityStandard"},
		{name: "a new value of a label the template had", old: literal(privileged), updated: literal(map[string]string{"security.deckhouse.io/pod-policy": "baseline"}), mentions: "security.deckhouse.io/pod-policy"},
		{name: "a literal replacing a reference", old: v1alpha2.FromParamRef[map[string]string]("namespace.labels"), updated: literal(privileged), mentions: "security.deckhouse.io/pod-policy"},
		{name: "an edit that keeps a label the template had", old: literal(privileged), updated: literal(privilegedAndTeam), allowed: true, warns: "security.deckhouse.io/pod-policy (set it through spec.podSecurityStandard)"},
		{name: "removing a label the template had", old: literal(privilegedAndTeam), updated: literal(map[string]string{"team": "a"}), allowed: true},
	}
	for _, tt := range updates {
		t.Run("update: "+tt.name, func(t *testing.T) {
			resp := newValidator(t).Handle(ctx, updateRequest(t, withLabels(tt.old), withLabels(tt.updated)))
			require.Equal(t, tt.allowed, resp.Allowed, resp.Result.Message)
			if tt.mentions != "" {
				assert.Contains(t, resp.Result.Message, "sets labels the module owns")
				assert.Contains(t, resp.Result.Message, tt.mentions)
			}
			if tt.warns == "" {
				assert.Empty(t, resp.Warnings)
				return
			}
			require.Len(t, resp.Warnings, 1)
			assert.Contains(t, resp.Warnings[0], "its namespaces do not get them")
			assert.Contains(t, resp.Warnings[0], tt.warns)
		})
	}
}

// TestHandle_InlineGrantSelectors: the template schema accepts selector shapes the
// ClusterResourceGrantPolicy schema refuses, and the managed policy built from them is then rejected
// at admission on every reconcile, with nothing on the template to say why.
func TestHandle_InlineGrantSelectors(t *testing.T) {
	ctx := context.Background()

	withResources := func(res ...grantsv1alpha1.GrantResource) *v1alpha2.ProjectTemplate {
		tmpl := structuredTemplate("tmpl")
		tmpl.Spec.Resources = res
		return tmpl
	}

	t.Run("an empty allowedSelector is refused", func(t *testing.T) {
		resp := newValidator(t).Handle(ctx, createRequest(t, withResources(grantsv1alpha1.GrantResource{
			ResourceName: "storageclasses", AllowedSelector: &metav1.LabelSelector{},
		})))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "empty selector")
	})

	t.Run("an empty deniedSelector is refused", func(t *testing.T) {
		resp := newValidator(t).Handle(ctx, createRequest(t, withResources(grantsv1alpha1.GrantResource{
			ResourceName: "storageclasses", DeniedSelector: &metav1.LabelSelector{},
		})))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "deniedSelector")
	})

	t.Run("In without values is refused", func(t *testing.T) {
		resp := newValidator(t).Handle(ctx, createRequest(t, withResources(grantsv1alpha1.GrantResource{
			ResourceName: "storageclasses",
			AllowedSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: "tier", Operator: metav1.LabelSelectorOpIn,
			}}},
		})))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "not a valid selector")
	})

	t.Run("a usable selector and an entry without one are accepted", func(t *testing.T) {
		resp := newValidator(t).Handle(ctx, createRequest(t, withResources(
			grantsv1alpha1.GrantResource{
				ResourceName:    "storageclasses",
				AllowedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shared": "true"}},
			},
			grantsv1alpha1.GrantResource{ResourceName: "ingressclasses", Allowed: []string{"nginx"}},
		)))
		assert.True(t, resp.Allowed, resp.Result)
	})
}

// TestHandle_DeleteTemplateStillRendered: a project that switched to a template that has not rendered
// yet no longer carries the old template's label, but its namespaces do, and the policies of the old
// template select them by it. The old template stays until nothing is rendered from it.
func TestHandle_DeleteTemplateStillRendered(t *testing.T) {
	ctx := context.Background()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: map[string]string{v1alpha3.ResourceLabelTemplate: "tmpl-v2"}},
	}
	rendered := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: map[string]string{
		v1alpha3.ResourceLabelTemplate: "tmpl",
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		v1alpha3.ResourceLabelProject:  "proj",
	}}}
	v := newValidator(t, project, rendered)

	old, err := json.Marshal(structuredTemplate("tmpl"))
	require.NoError(t, err)
	resp := v.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		OldObject: runtime.RawExtension{Raw: old},
	}})
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "still rendered from it")
	assert.Contains(t, resp.Result.Message, "'proj' project")
}

// TestHandle_DeleteTemplateNothingRendered: the delete guard counts only live project namespaces that
// carry the template label. A template nothing is rendered from, a namespace of another template, a
// namespace the module does not own, and one that is being deleted do not hold it.
func TestHandle_DeleteTemplateNothingRendered(t *testing.T) {
	ctx := context.Background()
	namespace := func(name, template, heritage string, terminating bool) *corev1.Namespace {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{
			v1alpha3.ResourceLabelTemplate: template,
			v1alpha3.ResourceLabelProject:  name,
		}}}
		if heritage != "" {
			ns.Labels[v1alpha3.ResourceLabelHeritage] = heritage
		}
		if terminating {
			ns.Finalizers = []string{"kubernetes"}
			ns.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
		}
		return ns
	}

	for _, tt := range []struct {
		name string
		objs []client.Object
	}{
		{name: "no namespaces"},
		{name: "a namespace of another template", objs: []client.Object{namespace("other", "tmpl-v2", v1alpha3.ResourceHeritageMultitenancy, false)}},
		{name: "a namespace the module does not own", objs: []client.Object{namespace("foreign", "tmpl", "", false)}},
		{name: "a namespace being deleted", objs: []client.Object{namespace("leaving", "tmpl", v1alpha3.ResourceHeritageMultitenancy, true)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := newValidator(t, tt.objs...)
			old, err := json.Marshal(structuredTemplate("tmpl"))
			require.NoError(t, err)
			resp := v.Handle(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
				OldObject: runtime.RawExtension{Raw: old},
			}})
			assert.True(t, resp.Allowed, "%v", resp.Result)
		})
	}
}

// TestHandle_DeleteTemplateLeftoverNamespace: a live namespace rendered from the template whose project is
// gone holds the template too, and the refusal says what to do.
func TestHandle_DeleteTemplateLeftoverNamespace(t *testing.T) {
	leftover := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "gone", Labels: map[string]string{
		v1alpha3.ResourceLabelTemplate: "tmpl",
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		v1alpha3.ResourceLabelProject:  "gone",
	}}}
	v := newValidator(t, leftover)

	old, err := json.Marshal(structuredTemplate("tmpl"))
	require.NoError(t, err)
	resp := v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		OldObject: runtime.RawExtension{Raw: old},
	}})
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "the project is gone; delete the namespace first")
}

func helmTemplate(name string) *v1alpha2.ProjectTemplate {
	return &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1alpha2.ProjectTemplateSpec{ResourcesTemplate: "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"},
	}
}

// A template renders from its Helm text or from its structured fields. The fields that render no
// object -- the grants, the title -- go with either.
func TestHandle_ResourcesTemplateWithStructuredFields(t *testing.T) {
	ctx := context.Background()
	v := newValidator(t, libraryPolicy("lib"))

	both := helmTemplate("both")
	both.Spec.PodSecurityStandard = v1alpha2.LiteralParam(v1alpha2.PodSecurityStandardBaseline)
	both.Spec.NetworkPolicy = &v1alpha2.NetworkPolicySpec{Mode: v1alpha2.LiteralParam(v1alpha2.NetworkPolicyModeIsolated)}
	resp := v.Handle(ctx, updateRequest(t, helmTemplate("both"), both))
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "resourcesTemplate together with fields that render objects: podSecurityStandard, networkPolicy")

	// a template with a Helm text is created through v1alpha1 alone, which has no structured fields
	resp = v.Handle(ctx, createRequest(t, both))
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "cannot get a new resourcesTemplate", "a create through v1alpha2 is refused for the text alone")

	// manifests renders objects too, and next to a Helm text it would go unused
	withManifests := helmTemplate("manifests")
	withManifests.Spec.Manifests = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: settings\n"
	resp = v.Handle(ctx, updateRequest(t, helmTemplate("manifests"), withManifests))
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "resourcesTemplate together with fields that render objects: manifests")

	withGrants := helmTemplate("grants")
	withGrants.Spec.Title = "Grants"
	withGrants.Spec.GrantPolicies = []string{"lib"}
	withGrants.Spec.Resources = []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}
	resp = v.Handle(ctx, updateRequest(t, helmTemplate("grants"), withGrants))
	assert.True(t, resp.Allowed, resp.Result)

	// an empty stanza renders nothing
	emptyStanza := helmTemplate("empty-stanza")
	emptyStanza.Spec.NetworkPolicy = &v1alpha2.NetworkPolicySpec{}
	resp = v.Handle(ctx, updateRequest(t, helmTemplate("empty-stanza"), emptyStanza))
	assert.True(t, resp.Allowed, resp.Result)
}

// Emptying the resourcesTemplate of a template that projects use would leave each of them its
// namespace alone and nobody managing what the text rendered. Setting structured fields in the same
// request is let through, and so is a template nobody uses.
func TestHandle_ResourcesTemplateRemoval(t *testing.T) {
	ctx := context.Background()
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: map[string]string{v1alpha3.ResourceLabelTemplate: "used"}},
	}
	v := newValidator(t, project)
	blank := func(name string) *v1alpha2.ProjectTemplate {
		template := helmTemplate(name)
		template.Spec.ResourcesTemplate = " \n"
		return template
	}

	resp := v.Handle(ctx, updateRequest(t, helmTemplate("used"), blank("used")))
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "used in the 'proj' project")
	assert.Contains(t, resp.Result.Message, "leaves every object of the Helm template that the new render lacks in place",
		"a switch keeps what it drops")
	assert.Contains(t, resp.Result.Message, "set the structured fields or manifests in the same request")
	assert.Contains(t, resp.Result.Message, "until manifests renders it", "manifests takes over what a switch keeps")

	rewritten := blank("used")
	rewritten.Spec.PodSecurityStandard = v1alpha2.LiteralParam(v1alpha2.PodSecurityStandardBaseline)
	resp = v.Handle(ctx, updateRequest(t, helmTemplate("used"), rewritten))
	assert.True(t, resp.Allowed, resp.Result)

	toManifests := blank("used")
	toManifests.Spec.Manifests = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"
	resp = v.Handle(ctx, updateRequest(t, helmTemplate("used"), toManifests))
	assert.True(t, resp.Allowed, "a rewrite with manifests in place of the Helm text passes: %v", resp.Result)

	resp = v.Handle(ctx, updateRequest(t, helmTemplate("unused"), blank("unused")))
	assert.True(t, resp.Allowed, resp.Result)

	resp = v.Handle(ctx, updateRequest(t, blank("used"), blank("used")))
	assert.True(t, resp.Allowed, "a template that had no Helm text loses nothing")
}

// TestHandle_TemplateUseBeforeTheLabel: the controller labels a project with its template after the
// project is written, and the cache may not have the project yet. A project that names the template
// in its spec, read from the API server, holds the template against emptying its resourcesTemplate
// and against a delete, and so does one whose label still names it after a move.
func TestHandle_TemplateUseBeforeTheLabel(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha2.AddToScheme(scheme))
	require.NoError(t, v1alpha3.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	created := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "just-created"},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "used"},
	}
	movedAway := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "moved-away", Labels: map[string]string{v1alpha3.ResourceLabelTemplate: "left"}},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: "other"},
	}
	v := &validator{
		client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		reader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(created, movedAway).Build(),
	}
	blank := func(name string) *v1alpha2.ProjectTemplate {
		template := helmTemplate(name)
		template.Spec.ResourcesTemplate = ""
		return template
	}
	deleteRequest := func(t *testing.T, template *v1alpha2.ProjectTemplate) admission.Request {
		t.Helper()
		old, err := json.Marshal(template)
		require.NoError(t, err)
		return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: admissionv1.Delete, OldObject: runtime.RawExtension{Raw: old}}}
	}

	for name, project := range map[string]string{"used": "just-created", "left": "moved-away"} {
		resp := v.Handle(ctx, updateRequest(t, helmTemplate(name), blank(name)))
		require.False(t, resp.Allowed, name)
		assert.Contains(t, resp.Result.Message, "used in the '"+project+"' project")

		resp = v.Handle(ctx, deleteRequest(t, helmTemplate(name)))
		require.False(t, resp.Allowed, name)
		assert.Contains(t, resp.Result.Message, "it is used in the '"+project+"' project")
	}

	resp := v.Handle(ctx, deleteRequest(t, helmTemplate("unused")))
	assert.True(t, resp.Allowed, resp.Result)
}

// A request through v1alpha1 carries none of the fields only v1alpha2 has, and its oldObject is read
// through v1alpha1 as well, so it shows none either. The webhook reads the stored template at
// v1alpha2 and refuses the request when there is something it would erase.
func TestHandle_V1alpha1Erasure(t *testing.T) {
	ctx := context.Background()

	structured := structuredTemplate("structured")
	structured.Spec.Title = "Structured"
	grants := helmTemplate("grants")
	grants.Spec.ResourcesTemplate = ""
	grants.Spec.Resources = []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses"}}
	grants.Spec.GrantPolicies = []string{"lib"}
	manifests := helmTemplate("manifests")
	manifests.Spec.ResourcesTemplate = ""
	manifests.Spec.Manifests = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"
	v := newValidator(t, structured, grants, manifests, helmTemplate("helm"), libraryPolicy("lib"))

	// what the API server hands the webhook for a request through v1alpha1: both objects as v1alpha1
	// sees them, converted to v1alpha2
	v1alpha1Request := func(t *testing.T, name string, changed func(*v1alpha2.ProjectTemplate)) admission.Request {
		t.Helper()
		stored := new(v1alpha2.ProjectTemplate)
		require.NoError(t, v.reader.Get(ctx, client.ObjectKey{Name: name}, stored))
		old := &v1alpha2.ProjectTemplate{ObjectMeta: *stored.ObjectMeta.DeepCopy()}
		old.Spec.Description = stored.Spec.Description
		old.Spec.ParametersSchema = stored.Spec.ParametersSchema
		old.Spec.ResourcesTemplate = stored.Spec.ResourcesTemplate
		updated := old.DeepCopy()
		changed(updated)

		req := updateRequest(t, old, updated)
		req.RequestKind = v1alpha1Kind.DeepCopy()
		return req
	}
	addLabel := func(template *v1alpha2.ProjectTemplate) {
		template.Labels = map[string]string{"team": "platform"}
	}

	t.Run("a label through v1alpha1 on a template with structured fields is refused", func(t *testing.T) {
		resp := v.Handle(ctx, v1alpha1Request(t, "structured", addLabel))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "fields that deckhouse.io/v1alpha1 cannot describe: title, podSecurityStandard")
	})

	t.Run("a label through v1alpha1 on a template with grants only is refused", func(t *testing.T) {
		resp := v.Handle(ctx, v1alpha1Request(t, "grants", addLabel))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "cannot describe: resources, grantPolicies")
	})

	t.Run("a label through v1alpha1 on a template with manifests is refused", func(t *testing.T) {
		resp := v.Handle(ctx, v1alpha1Request(t, "manifests", addLabel))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "cannot describe: manifests")
	})

	t.Run("a Helm template is changed through v1alpha1", func(t *testing.T) {
		resp := v.Handle(ctx, v1alpha1Request(t, "helm", func(template *v1alpha2.ProjectTemplate) {
			template.Spec.ResourcesTemplate += "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: s\n"
		}))
		assert.True(t, resp.Allowed, resp.Result)
	})

	t.Run("a template that changed since oldObject is refused for a retry", func(t *testing.T) {
		req := v1alpha1Request(t, "helm", addLabel)
		old := new(v1alpha2.ProjectTemplate)
		require.NoError(t, json.Unmarshal(req.OldObject.Raw, old))
		old.ResourceVersion = "1"
		raw, err := json.Marshal(old)
		require.NoError(t, err)
		req.OldObject.Raw = raw

		resp := v.Handle(ctx, req)
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "retry the request")
		assert.EqualValues(t, http.StatusConflict, resp.Result.Code, "a conflict is what clients retry on")
		assert.Equal(t, metav1.StatusReasonConflict, resp.Result.Reason)
	})

	t.Run("a request through v1alpha2 carries the structured fields and passes", func(t *testing.T) {
		updated := structured.DeepCopy()
		addLabel(updated)
		resp := v.Handle(ctx, updateRequest(t, structured.DeepCopy(), updated))
		assert.True(t, resp.Allowed, resp.Result)
	})

	// there is no stored template to read yet, and nothing to erase
	t.Run("a template is created through v1alpha1", func(t *testing.T) {
		req := createRequest(t, helmTemplate("new"))
		req.RequestKind = v1alpha1Kind.DeepCopy()
		resp := v.Handle(ctx, req)
		assert.True(t, resp.Allowed, resp.Result)
	})
}

// A template gets a new resourcesTemplate through v1alpha1 only. Through v1alpha2 a create with one
// and an update that sets one where the stored template has none are refused, and the refusal says
// where the objects go instead. A template that has one keeps it and can change it through either
// version, and a blank one counts as none.
func TestHandle_NewResourcesTemplateThroughV1alpha2(t *testing.T) {
	ctx := context.Background()
	blank := helmTemplate("blank")
	blank.Spec.ResourcesTemplate = " \n"
	blank.Spec.Description = "no text yet"
	v := newValidator(t, helmTemplate("helm"), blank)

	stored := func(t *testing.T, name string) *v1alpha2.ProjectTemplate {
		t.Helper()
		template := new(v1alpha2.ProjectTemplate)
		require.NoError(t, v.reader.Get(ctx, client.ObjectKey{Name: name}, template))
		return template
	}
	withText := func(template *v1alpha2.ProjectTemplate, text string) *v1alpha2.ProjectTemplate {
		changed := template.DeepCopy()
		changed.Spec.ResourcesTemplate = text
		return changed
	}
	const text = "---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: s\n"

	t.Run("a create through v1alpha2 with a text is refused", func(t *testing.T) {
		resp := v.Handle(ctx, createRequest(t, helmTemplate("new")))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "the 'new' project template cannot get a new resourcesTemplate through deckhouse.io/v1alpha2")
		assert.Contains(t, resp.Result.Message, "written through deckhouse.io/v1alpha1")
		assert.Contains(t, resp.Result.Message, "the structured fields")
		assert.Contains(t, resp.Result.Message, "manifests")
		assert.Contains(t, resp.Result.Message,
			"a template that still needs a Helm text is created through deckhouse.io/v1alpha1, and its title, resources or grantPolicies are added through deckhouse.io/v1alpha2 afterwards",
			"the refusal names the two writes of a Helm-text template with fields of v1alpha2")
	})

	t.Run("a create through v1alpha2 with a blank text passes", func(t *testing.T) {
		resp := v.Handle(ctx, createRequest(t, withText(helmTemplate("new"), " \n")))
		assert.True(t, resp.Allowed, resp.Result)
	})

	t.Run("an update through v1alpha2 that sets a text where there was none is refused", func(t *testing.T) {
		old := stored(t, "blank")
		resp := v.Handle(ctx, updateRequest(t, old, withText(old, text)))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "cannot get a new resourcesTemplate")
	})

	t.Run("an update through v1alpha2 changes the text a template has", func(t *testing.T) {
		old := stored(t, "helm")
		resp := v.Handle(ctx, updateRequest(t, old, withText(old, old.Spec.ResourcesTemplate+text)))
		assert.True(t, resp.Allowed, resp.Result)
	})

	t.Run("an update through v1alpha2 that keeps the text passes", func(t *testing.T) {
		old := stored(t, "helm")
		labelled := old.DeepCopy()
		labelled.Labels = map[string]string{"team": "platform"}
		resp := v.Handle(ctx, updateRequest(t, old, labelled))
		assert.True(t, resp.Allowed, resp.Result)
	})

	t.Run("a create through v1alpha1 with a text passes", func(t *testing.T) {
		req := createRequest(t, helmTemplate("new"))
		req.RequestKind = v1alpha1Kind.DeepCopy()
		resp := v.Handle(ctx, req)
		assert.True(t, resp.Allowed, resp.Result)
	})

	t.Run("an update through v1alpha1 that sets a text where there was none passes", func(t *testing.T) {
		old := stored(t, "blank")
		req := updateRequest(t, old, withText(old, text))
		req.RequestKind = v1alpha1Kind.DeepCopy()
		resp := v.Handle(ctx, req)
		assert.True(t, resp.Allowed, resp.Result)
	})
}
