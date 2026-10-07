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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha2"
)

// A template with a resourcesTemplate string is rendered through the legacy helm engine; a template
// without one is rendered natively from its structured fields.
func TestIsStructured(t *testing.T) {
	assert.False(t, isStructured(&v1alpha2.ProjectTemplate{Spec: v1alpha2.ProjectTemplateSpec{ResourcesTemplate: "---\nkind: Namespace\n"}}))
	assert.True(t, isStructured(&v1alpha2.ProjectTemplate{Spec: v1alpha2.ProjectTemplateSpec{ResourcesTemplate: "  \n "}}), "whitespace-only resourcesTemplate is treated as empty -> structured")
	assert.True(t, isStructured(&v1alpha2.ProjectTemplate{Spec: v1alpha2.ProjectTemplateSpec{PodSecurityStandard: v1alpha2.LiteralParam("Baseline")}}))
}

// legacyTemplate projects the parametersSchema (and any resourcesTemplate) onto the v1alpha1 shape used
// for validation and the legacy render path; structured fields are intentionally not carried over.
func TestLegacyTemplate(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}}
	in := &v1alpha2.ProjectTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tmpl", Generation: 4},
		Spec: v1alpha2.ProjectTemplateSpec{
			Description:       "desc",
			ResourcesTemplate: "---\nkind: ConfigMap\n",
			ParametersSchema:  v1alpha2.ParametersSchema{OpenAPIV3Schema: schema},
		},
	}

	out := LegacyTemplate(in)
	assert.Equal(t, "tmpl", out.Name)
	assert.EqualValues(t, 4, out.Generation)
	assert.Equal(t, "desc", out.Spec.Description)
	assert.Equal(t, "---\nkind: ConfigMap\n", out.Spec.ResourcesTemplate)
	assert.Equal(t, schema, out.Spec.ParametersSchema.OpenAPIV3Schema)
}
