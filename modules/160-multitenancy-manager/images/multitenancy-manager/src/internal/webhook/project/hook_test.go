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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	projectmanager "controller/internal/manager/project"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

func TestValidateStandardFields(t *testing.T) {
	cases := []struct {
		name    string
		project *v1alpha3.Project
		denied  bool
	}{
		{
			name:    "empty is valid",
			project: &v1alpha3.Project{},
		},
		{
			name: "valid administrators and quota",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "Group", Name: "team"}},
				Quota: corev1.ResourceList{
					"requests.cpu":    resource.MustParse("2"),
					"requests.memory": resource.MustParse("1Gi"),
					"limits.memory":   resource.MustParse("512Mi"),
					"pods":            resource.MustParse("10"),
				},
			}},
		},
		{
			name: "cpu and pods may be bare numbers",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{
					"requests.cpu": resource.MustParse("5"),
					"limits.cpu":   resource.MustParse("500m"),
					"pods":         resource.MustParse("10"),
				},
			}},
		},
		{
			name: "memory with decimal SI unit is allowed",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.memory": resource.MustParse("1G")},
			}},
		},
		{
			name: "bare memory is denied",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.memory": resource.MustParse("5")},
			}},
			denied: true,
		},
		{
			name: "bare storage is denied",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.storage": resource.MustParse("5")},
			}},
			denied: true,
		},
		{
			name: "ephemeral-storage without unit is denied",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.ephemeral-storage": resource.MustParse("100")},
			}},
			denied: true,
		},
		{
			name: "memory milli suffix is denied",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"limits.memory": resource.MustParse("1000m")},
			}},
			denied: true,
		},
		{
			name: "hugepages count may be bare",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Quota: corev1.ResourceList{"requests.hugepages-2Mi": resource.MustParse("5")},
			}},
		},
		{
			name: "invalid administrator kind",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "ServiceAccount", Name: "robot"}},
			}},
			denied: true,
		},
		{
			name: "empty administrator name",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: ""}},
			}},
			denied: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := validateStandardFields(tc.project)
			if tc.denied {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}
		})
	}
}

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha2.AddToScheme, v1alpha3.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c}
}

// The template is read at the version the API serves, and only projected onto the legacy shape
// afterwards. Reading it at v1alpha1 worked only while that version was served -- the apiserver
// converted the stored object on every call -- and the day it stopped being served, every project
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
	require.NotNil(t, template, "the template was not found at the served version")
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

// The two name rules the webhook enforces on create: the platform prefixes are reserved, and the
// "<project>-*" name space belongs to the additional namespaces of an existing project in both
// directions. The checks existed; these are the tests the design promised for them.
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

	t.Run("platform prefixes are reserved", func(t *testing.T) {
		v := newValidator(t)
		for _, name := range []string{"d8-system", "d8-foo", "kube-system", "kube-foo"} {
			resp := v.Handle(context.Background(), createReq(t, name))
			assert.False(t, resp.Allowed, name)
			assert.Contains(t, resp.Result.Message, "cannot start with 'd8-' or 'kube-'", name)
		}
	})

	t.Run("a name under an existing project's additional-namespace space is refused", func(t *testing.T) {
		existing := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		v := newValidator(t, existing)
		resp := v.Handle(context.Background(), createReq(t, "foo-bar"))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `project name "foo-bar" conflicts with project "foo"`)
	})

	t.Run("a name whose additional-namespace space already holds a project is refused", func(t *testing.T) {
		existing := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo-bar"}}
		v := newValidator(t, existing)
		resp := v.Handle(context.Background(), createReq(t, "foo"))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `project name "foo" conflicts with project "foo-bar"`)
	})

	t.Run("an unrelated name is allowed beside both", func(t *testing.T) {
		v := newValidator(t, &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}})
		resp := v.Handle(context.Background(), createReq(t, "foobar"))
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
