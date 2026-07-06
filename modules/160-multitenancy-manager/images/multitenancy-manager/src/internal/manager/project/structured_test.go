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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

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

	out := legacyTemplate(in)
	assert.Equal(t, "tmpl", out.Name)
	assert.EqualValues(t, 4, out.Generation)
	assert.Equal(t, "desc", out.Spec.Description)
	assert.Equal(t, "---\nkind: ConfigMap\n", out.Spec.ResourcesTemplate)
	assert.Equal(t, schema, out.Spec.ParametersSchema.OpenAPIV3Schema)
}

// The built-in templates must be schema-based v1alpha2 documents with no Helm resourcesTemplate, wiring
// every per-project knob to a fromParam leaf while keeping the parametersSchema as the parameter contract.
func TestBuiltinTemplatesAreStructured(t *testing.T) {
	for _, file := range []string{"default.yaml", "secure.yaml", "secure-with-dedicated-nodes.yaml"} {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", file))
			require.NoError(t, err)

			tmpl := new(v1alpha2.ProjectTemplate)
			require.NoError(t, yaml.Unmarshal(raw, tmpl))

			assert.Equal(t, "deckhouse.io/v1alpha2", tmpl.APIVersion)
			assert.Empty(t, tmpl.Spec.ResourcesTemplate, "built-in templates must not carry a Helm string")
			assert.True(t, isStructured(tmpl))

			// the per-project knobs are wired to their parameters via fromParam
			assert.Equal(t, "podSecurityProfile", tmpl.Spec.PodSecurityStandard.Ref())
			require.NotNil(t, tmpl.Spec.NetworkPolicy)
			assert.Equal(t, "networkPolicy", tmpl.Spec.NetworkPolicy.Mode.Ref())
			require.NotNil(t, tmpl.Spec.Features)
			assert.Equal(t, "extendedMonitoringEnabled", tmpl.Spec.Features.Monitoring.Ref())

			// the parameter contract is preserved
			props, ok := tmpl.Spec.ParametersSchema.OpenAPIV3Schema["properties"].(map[string]any)
			require.True(t, ok)
			assert.Contains(t, props, "podSecurityProfile")
			assert.Contains(t, props, "networkPolicy")
			assert.Contains(t, props, "namespace")
		})
	}
}
