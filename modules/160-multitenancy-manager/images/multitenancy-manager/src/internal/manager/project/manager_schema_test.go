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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// A template whose parametersSchema still declares and requires the administrators and
// resourceQuota parameters validates a project that keeps the two in its standard fields, and the
// project is rendered from its own parameters.
func TestHandleTemplateValidatesAgainstTheParametersLayout(t *testing.T) {
	tmpl := structuredTemplate()
	tmpl.Spec.ParametersSchema.OpenAPIV3Schema = map[string]any{
		"type":     "object",
		"required": []any{"administrators", "resourceQuota"},
		"properties": map[string]any{
			"administrators": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"subject": map[string]any{"type": "string"},
						"name":    map[string]any{"type": "string"},
					},
				},
			},
			"resourceQuota": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"requests": map[string]any{
						"type":       "object",
						"properties": map[string]any{"cpu": map[string]any{"type": "string"}},
					},
				},
			},
		},
	}
	project := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec: v1alpha3.ProjectSpec{
			ProjectTemplateName: "tmpl",
			Administrators:      []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			Quota:               corev1.ResourceList{"requests.cpu": resource.MustParse("2")},
		},
	}
	m, _ := newManager(t, tmpl, project)
	fh := &fakeHelmClient{}
	m.helmClient = fh

	done, err := m.handleTemplate(context.Background(), project)
	require.NoError(t, err)
	assert.False(t, done)
	assert.False(t, project.IsConditionFalse(v1alpha3.ProjectConditionProjectValidated),
		conditionByType(project, v1alpha3.ProjectConditionProjectValidated))
	assert.Equal(t, 1, fh.upgradeCalls, "the project renders")
	assert.Nil(t, project.Spec.Parameters, "validation does not write the parameters back")
}

// The v1alpha2 -> v1alpha3 conversion leaves administrators or resourceQuota in spec.parameters when it
// cannot give them back as they were, and the built-in default template declares neither any more. A
// project of that template from the previous release, such as e2e-bidef-zero and e2e-bidef-noadm of
// the upgrade e2e test, validates, renders and gets the object of the standard field it has,
// d8-administrators or d8-project-quota, and none for the one left in the parameters.
func TestHandleReconcilesWhatTheConversionLeavesInTheParameters(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "default.yaml"))
	require.NoError(t, err)
	tmpl := new(v1alpha2.ProjectTemplate)
	require.NoError(t, yaml.Unmarshal(raw, tmpl))
	require.True(t, isStructured(tmpl), "the built-in default template is structured")

	tests := []struct {
		name string
		spec v1alpha3.ProjectSpec
		// administrators and quota are what the standard-field objects must hold, nil for none
		administrators []string
		quota          corev1.ResourceList
	}{
		{
			name: "a quota with the number 0",
			spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}},
				Parameters: map[string]any{
					"podSecurityProfile": "Baseline",
					"resourceQuota":      map[string]any{"requests": map[string]any{"cpu": float64(0), "memory": "2Gi"}},
				},
			},
			administrators: []string{"alice@example.com"},
		},
		{
			name: "an empty administrators list",
			spec: v1alpha3.ProjectSpec{
				Quota:      corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters: map[string]any{"administrators": []any{}},
			},
			quota: corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}, Spec: *tt.spec.DeepCopy()}
			project.Spec.ProjectTemplateName = tmpl.Name
			m, c := newManager(t, tmpl.DeepCopy(), project)
			fh := &fakeHelmClient{applyResult: helm.ReleaseOutcome{Applied: true}}
			m.helmClient = fh

			_, err := m.Handle(ctx, project)
			require.NoError(t, err)

			got := new(v1alpha3.Project)
			require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "team"}, got))
			assert.Equal(t, v1alpha3.ProjectStateDeployed, got.Status.State, "conditions: %+v", got.Status.Conditions)
			for _, condition := range []string{
				v1alpha3.ProjectConditionProjectValidated,
				v1alpha3.ProjectConditionProjectResourcesUpgraded,
				v1alpha3.ProjectConditionStandardFieldsApplied,
			} {
				cond := conditionByType(got, condition)
				require.NotNil(t, cond, condition)
				assert.Equal(t, corev1.ConditionTrue, cond.Status, condition)
			}
			assert.Equal(t, 1, fh.upgradeCalls, "the project renders")

			binding := new(v1alpha3.ProjectRoleBinding)
			err = c.Get(ctx, client.ObjectKey{Namespace: "team", Name: v1alpha3.ProjectAdministratorsBinding}, binding)
			if tt.administrators == nil {
				assert.True(t, apierrors.IsNotFound(err), "no administrators binding: %v", err)
			} else {
				require.NoError(t, err)
				names := make([]string, 0, len(binding.Spec.Subjects))
				for _, subject := range binding.Spec.Subjects {
					names = append(names, subject.Name)
				}
				assert.Equal(t, tt.administrators, names)
			}

			quota := new(corev1.ResourceQuota)
			err = c.Get(ctx, client.ObjectKey{Namespace: "team", Name: v1alpha3.ProjectQuotaName}, quota)
			if tt.quota == nil {
				assert.True(t, apierrors.IsNotFound(err), "no project quota: %v", err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.quota, quota.Spec.Hard)
			}

			want, err := json.Marshal(tt.spec.Parameters)
			require.NoError(t, err)
			kept, err := json.Marshal(got.Spec.Parameters)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), string(kept), "the parameters stay as the conversion left them")
		})
	}
}
