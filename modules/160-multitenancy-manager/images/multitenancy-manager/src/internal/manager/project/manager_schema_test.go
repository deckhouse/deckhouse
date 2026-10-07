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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha3"
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
