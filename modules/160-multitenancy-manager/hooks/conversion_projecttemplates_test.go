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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The golden fixtures pin both conversions of the projecttemplates hook object by object: the version
// bump on the way up, the backfill on the way down, and everything the hook leaves alone.
func TestProjectTemplateConversionGolden(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"v1alpha1_to_v1alpha2",
		"v1alpha2_to_v1alpha1",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			testConversionGolden(t, templateConversionHook, filepath.Join(conversionFixtures, "projecttemplates", path+".yaml"))
		})
	}
}

// The apiserver keeps asking for the conversion of whatever v1alpha1 objects it holds, and a
// template with no spec at all is a shape it can hold. The conversion must answer, not fail.
func TestProjectTemplateConversionSurvivesAMissingSpec(t *testing.T) {
	t.Parallel()

	bare := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
	}

	up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", bare)
	assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
	_, hasSpec := up["spec"]
	assert.False(t, hasSpec, "a missing spec is not conjured")

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
	})
	assert.Equal(t, "deckhouse.io/v1alpha1", down["apiVersion"])
	assert.Equal(t, "", specField(down, "resourcesTemplate"), "v1alpha1 requires the field, so it is backfilled")
}

// A ProjectTemplate keeps the fields both versions describe across the version bump, the Helm
// resourcesTemplate among them, so a template written as v1alpha1 reads back as it was written. The
// structured fields of v1alpha2 are pruned on the way down and cannot come back. The apiserver asks
// for the conversion on every v1alpha1 request, and it once went unanswered: removing this hook left
// a live cluster failing about one conversion a second.
func TestProjectTemplateVersionBump(t *testing.T) {
	t.Parallel()

	template := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
		"spec": map[string]any{
			"description":       "a template",
			"resourcesTemplate": "---\napiVersion: v1\nkind: Namespace\n",
			"parametersSchema":  map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
		},
	}

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", template)
	assert.Equal(t, "deckhouse.io/v1alpha1", down["apiVersion"])
	assert.Equal(t, specField(template, "description"), specField(down, "description"))
	assert.Equal(t, specField(template, "resourcesTemplate"), specField(down, "resourcesTemplate"))

	up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", down)
	assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
	assert.Equal(t, template["spec"], up["spec"])
}

// A structured template has neither of the two fields the v1alpha1 schema requires, so they are
// backfilled rather than left missing. The apiserver checks only the metadata of what a conversion
// returns, but a client that writes the template back through v1alpha1 is validated against that
// schema.
func TestProjectTemplateBackfillsWhatV1alpha1Requires(t *testing.T) {
	t.Parallel()

	structured := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
		"spec":       map[string]any{"description": "structured only"},
	}

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", structured)
	assert.Equal(t, "", specField(down, "resourcesTemplate"))
	assert.Equal(t, map[string]any{"openAPIV3Schema": map[string]any{}}, specField(down, "parametersSchema"))
}
