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

package namespace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
	"controller/internal/render"
	"controller/internal/validate"
)

func TestTemplateFor(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "no rendered label at all",
			labels: nil,
			want:   TemplateSimple,
		},
		{
			name:   "only user labels",
			labels: map[string]string{"team": "blue"},
			want:   TemplateSimple,
		},
		{
			name:   "pod policy alone",
			labels: map[string]string{labelPodPolicy: "restricted"},
			want:   TemplateDefault,
		},
		{
			name:   "extended monitoring alone",
			labels: map[string]string{labelExtendedMonitoring: ""},
			want:   TemplateDefault,
		},
		{
			name:   "vulnerability scanning wins over the rest",
			labels: map[string]string{labelSecurityScanning: "", labelPodPolicy: "baseline"},
			want:   TemplateSecure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TemplateFor(namespace("foo", tt.labels, nil)))
		})
	}
}

func TestParametersFor(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		template    string
		want        map[string]any
	}{
		{
			name:     "simple without user metadata carries no parameters",
			template: TemplateSimple,
			want:     nil,
		},
		{
			name:        "simple mirrors user metadata only",
			labels:      map[string]string{"team": "blue", v1alpha3.ResourceLabelProject: "foo"},
			annotations: map[string]string{"owner": "alice"},
			template:    TemplateSimple,
			want: map[string]any{
				"namespace": map[string]any{
					"labels":      map[string]any{"team": "blue"},
					"annotations": map[string]any{"owner": "alice"},
				},
			},
		},
		{
			name:     "default leaves a namespace without a pod policy on the cluster default",
			labels:   map[string]string{labelExtendedMonitoring: ""},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileClusterDefault,
				"extendedMonitoringEnabled": true,
				"requiredRequests":          false,
			},
		},
		{
			name:     "default leaves a pod policy it cannot render to the namespace",
			labels:   map[string]string{labelPodPolicy: "Restricted"},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileClusterDefault,
				"extendedMonitoringEnabled": false,
				"requiredRequests":          false,
			},
		},
		{
			name:     "default reads the profile back from the rendered label",
			labels:   map[string]string{labelPodPolicy: "restricted"},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileRestricted,
				"extendedMonitoringEnabled": false,
				"requiredRequests":          false,
			},
		},
		{
			name:     "secure adds the scanning switch",
			labels:   map[string]string{labelSecurityScanning: "", labelPodPolicy: "baseline"},
			template: TemplateSecure,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileBaseline,
				"extendedMonitoringEnabled": false,
				"securityScanningEnabled":   true,
				"requiredRequests":          false,
			},
		},
		{
			name:     "rendered labels are not mirrored as user metadata",
			labels:   map[string]string{labelPodPolicy: "baseline", labelExtendedMonitoring: ""},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileBaseline,
				"extendedMonitoringEnabled": true,
				"requiredRequests":          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParametersFor(namespace("foo", tt.labels, tt.annotations), tt.template)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParametersFor_KeepsThePodPolicyOnBuiltinTemplates adopts a namespace end to end on the
// built-in template adoption picks for it: the parameters must validate against that template, and
// the rendered Namespace, applied the way the release applies it, must leave the pod-policy label
// exactly as the namespace had it. A namespace without the label keeps none, so the cluster default
// policy goes on applying to it.
func TestParametersFor_KeepsThePodPolicyOnBuiltinTemplates(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
	}{
		{name: "no pod policy", labels: map[string]string{labelExtendedMonitoring: ""}},
		{name: "no pod policy on the secure template", labels: map[string]string{labelSecurityScanning: ""}},
		{name: "baseline", labels: map[string]string{labelPodPolicy: "baseline"}},
		{name: "restricted", labels: map[string]string{labelPodPolicy: "restricted"}},
		{name: "privileged", labels: map[string]string{labelPodPolicy: "privileged"}},
		{name: "capitalised profile name", labels: map[string]string{labelPodPolicy: "Restricted"}},
		{name: "unknown value", labels: map[string]string{labelPodPolicy: "whatever"}},
		{name: "empty value", labels: map[string]string{labelPodPolicy: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			live := namespace("foo", tt.labels, nil)
			templateName := TemplateFor(live)
			tmpl := builtinTemplate(t, templateName)
			adopted := &v1alpha3.Project{
				ObjectMeta: metav1.ObjectMeta{Name: live.Name},
				Spec: v1alpha3.ProjectSpec{
					ProjectTemplateName: templateName,
					Parameters:          ParametersFor(live, templateName),
				},
			}
			require.NoError(t, validate.Project(adopted, tmpl), "adoption parameters must validate against the built-in template")

			manifests, err := render.Manifests(tmpl, adopted)
			require.NoError(t, err)
			got := applyAsRelease(t, live, renderedNamespace(t, manifests))

			want, had := tt.labels[labelPodPolicy]
			value, has := got.Labels[labelPodPolicy]
			assert.Equal(t, had, has, "the pod-policy label must be present exactly when the namespace had it")
			assert.Equal(t, want, value, "the pod-policy label must keep its value")
		})
	}
}

func TestPodSecurityProfile(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  string
	}{
		{name: "missing label stays on the cluster default", label: "", want: podSecurityProfileClusterDefault},
		{name: "unknown value is left to the namespace", label: "whatever", want: podSecurityProfileClusterDefault},
		{name: "capitalised profile name is left to the namespace", label: "Restricted", want: podSecurityProfileClusterDefault},
		{name: "mixed case is left to the namespace", label: "ReStRiCtEd", want: podSecurityProfileClusterDefault},
		{name: "baseline", label: "baseline", want: podSecurityProfileBaseline},
		{name: "restricted", label: "restricted", want: podSecurityProfileRestricted},
		{name: "privileged", label: "privileged", want: podSecurityProfilePrivileged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, podSecurityProfile(tt.label))
		})
	}
}

func TestFilterUserMeta(t *testing.T) {
	got := filterUserMeta(map[string]string{
		"team":                         "blue",
		"heritageSomething":            "keep",
		v1alpha3.ResourceLabelProject:  "foo",
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		"kubernetes.io/metadata.name":  "foo",
		"meta.helm.sh/release-name":    "foo",
		labelPodPolicy:                 "baseline",
		labelExtendedMonitoring:        "",
		labelSecurityScanning:          "",
	})
	assert.Equal(t, map[string]string{"team": "blue", "heritageSomething": "keep"}, got)
}

func TestFilterUserMeta_NilWhenNothingLeft(t *testing.T) {
	assert.Nil(t, filterUserMeta(map[string]string{v1alpha3.ResourceLabelProject: "foo"}))
	assert.Nil(t, filterUserMeta(nil))
}

func TestNeedsTemplate(t *testing.T) {
	tests := []struct {
		name    string
		project *v1alpha3.Project
		want    bool
	}{
		{
			name:    "project left over from the namespace-managed model",
			project: project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, ""),
			want:    true,
		},
		{
			name:    "project created without a template",
			project: project("foo", nil, ""),
			want:    true,
		},
		{
			name:    "both at once",
			project: project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, ""),
			want:    true,
		},
		{
			name:    "already migrated",
			project: project("foo", nil, TemplateSimple),
			want:    false,
		},
		{
			name:    "virtual project is platform-owned",
			project: project("deckhouse", map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}, "virtual"),
			want:    false,
		},
		{
			name: "virtual project without a template is still skipped",
			project: project("default", map[string]string{
				v1alpha3.ProjectLabelVirtualProject: "true",
			}, ""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, needsTemplate(tt.project))
		})
	}
}

// TestFilterUserMeta_KeepsPlacementAnnotations: the module owns the scheduler annotations on a
// project namespace, so a user cannot edit them there; they are rendered from the template's
// nodeSelector/tolerations, and adoption never picks a template that declares either. Dropping them
// on adoption would leave an adopted namespace pinned to nodes with no way to change it, so they are
// mirrored into the namespace parameter and stay editable through the Project.
func TestFilterUserMeta_KeepsPlacementAnnotations(t *testing.T) {
	got := filterUserMeta(map[string]string{
		naming.NodeSelectorAnnotation: "disk=ssd",
		naming.TolerationsAnnotation:  `[{"key":"dedicated"}]`,
		"meta.helm.sh/release-name":   "foo",
		"team":                        "blue",
	})
	assert.Equal(t, map[string]string{
		naming.NodeSelectorAnnotation: "disk=ssd",
		naming.TolerationsAnnotation:  `[{"key":"dedicated"}]`,
		"team":                        "blue",
	}, got)
}

// builtinTemplate reads the built-in template the controller installs under the given name.
// Every built-in template that takes its Pod Security Standard from the podSecurityProfile parameter
// accepts an empty profile, so that a project adopted with one can move to any of them.
func TestBuiltinTemplatesTakeAnEmptyProfile(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "templates", "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	checked := 0
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".yaml")
		tmpl := builtinTemplate(t, name)
		if tmpl.Spec.PodSecurityStandard.Ref() != "podSecurityProfile" {
			continue
		}
		checked++
		t.Run(name, func(t *testing.T) {
			project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}, Spec: v1alpha3.ProjectSpec{
				ProjectTemplateName: name,
				Parameters:          map[string]any{"podSecurityProfile": ""},
			}}
			require.NoError(t, validate.Project(project, tmpl))
		})
	}
	assert.GreaterOrEqual(t, checked, 3, "default, secure and secure-with-dedicated-nodes take the parameter")
}

func builtinTemplate(t *testing.T, name string) *v1alpha2.ProjectTemplate {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", name+".yaml"))
	require.NoError(t, err)

	tmpl := new(v1alpha2.ProjectTemplate)
	require.NoError(t, yaml.Unmarshal(raw, tmpl))

	return tmpl
}

// renderedNamespace picks the Namespace out of a multi-document render.
func renderedNamespace(t *testing.T, manifests string) *corev1.Namespace {
	t.Helper()

	for _, doc := range strings.Split(manifests, "---\n") {
		var kind metav1.TypeMeta
		require.NoError(t, yaml.Unmarshal([]byte(doc), &kind))
		if kind.Kind != "Namespace" {
			continue
		}

		ns := new(corev1.Namespace)
		require.NoError(t, yaml.Unmarshal([]byte(doc), ns))

		return ns
	}

	t.Fatal("the render must contain the project namespace")

	return nil
}

// applyAsRelease applies the rendered Namespace onto the live one the way the Helm release does: a
// three-way strategic merge whose last-applied state is the render itself when an existing object
// is taken into the release, and the previous render on every upgrade after that. Either way a key
// the render does not mention stays on the live object as it is.
func applyAsRelease(t *testing.T, live, rendered *corev1.Namespace) *corev1.Namespace {
	t.Helper()

	target, err := json.Marshal(rendered)
	require.NoError(t, err)
	current, err := json.Marshal(live)
	require.NoError(t, err)

	meta, err := strategicpatch.NewPatchMetaFromStruct(corev1.Namespace{})
	require.NoError(t, err)
	patch, err := strategicpatch.CreateThreeWayMergePatch(target, target, current, meta, true)
	require.NoError(t, err)
	merged, err := strategicpatch.StrategicMergePatch(current, patch, corev1.Namespace{})
	require.NoError(t, err)

	out := new(corev1.Namespace)
	require.NoError(t, json.Unmarshal(merged, out))

	return out
}
