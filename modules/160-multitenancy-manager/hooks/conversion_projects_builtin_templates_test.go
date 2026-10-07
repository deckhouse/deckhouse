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

package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// builtinTemplateFixtures holds, per built-in template, a v1alpha2 project as the Helm template path
// rendered it (project.yaml), the objects that render produced (resources.yaml) and the v1alpha3
// project the conversion makes of it (converted.yaml). The controller renders converted.yaml with the
// structured built-in template and checks it selects what resources.yaml selected; this test keeps
// converted.yaml what the shipped conversion really produces.
const builtinTemplateFixtures = "../images/multitenancy-manager/src/internal/helm/testdata/helm_template"

func TestProjectUpConversionOfBuiltinTemplateFixtures(t *testing.T) {
	t.Parallel()

	for _, template := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(template, func(t *testing.T) {
			t.Parallel()

			project := readYAMLObject(t, filepath.Join(builtinTemplateFixtures, template, "project.yaml"))
			expected := readYAMLObject(t, filepath.Join(builtinTemplateFixtures, template, "converted.yaml"))

			assert.Equal(t, expected, convert(t, "v1alpha2_to_v1alpha3", project))
		})
	}
}

func readYAMLObject(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	object := map[string]any{}
	require.NoError(t, yaml.Unmarshal(raw, &object))
	return object
}
