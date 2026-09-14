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
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
)

// The built-in templates are v1alpha2 documents wiring every per-project knob to a fromParam leaf while
// keeping the parametersSchema as the parameter contract.
func TestBuiltinTemplatesAreStructured(t *testing.T) {
	t.Run("simple.yaml", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "simple.yaml"))
		require.NoError(t, err)

		tmpl := new(v1alpha2.ProjectTemplate)
		require.NoError(t, yaml.Unmarshal(raw, tmpl))

		assert.Equal(t, "deckhouse.io/v1alpha2", tmpl.APIVersion)

		// the namespace labels and annotations stay per-project parameters
		require.NotNil(t, tmpl.Spec.NamespaceMetadata)
		assert.Equal(t, "namespace.labels", tmpl.Spec.NamespaceMetadata.Labels.Ref())
		assert.Equal(t, "namespace.annotations", tmpl.Spec.NamespaceMetadata.Annotations.Ref())
		props, ok := tmpl.Spec.ParametersSchema.OpenAPIV3Schema["properties"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, props, "namespace")
		// the required-requests policy is opt-in here: the template renders the namespace and nothing else
		required, ok := props["requiredRequests"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, false, required["default"])
	})

	for _, file := range []string{"default.yaml", "secure.yaml", "secure-with-dedicated-nodes.yaml"} {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", file))
			require.NoError(t, err)

			tmpl := new(v1alpha2.ProjectTemplate)
			require.NoError(t, yaml.Unmarshal(raw, tmpl))

			assert.Equal(t, "deckhouse.io/v1alpha2", tmpl.APIVersion)

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
