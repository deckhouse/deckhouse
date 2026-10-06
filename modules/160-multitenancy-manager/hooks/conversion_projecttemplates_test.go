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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The golden fixtures pin both conversions of the projecttemplates hook object by object: the mark and
// the body annotation on the way up, the backfill on the way down, and everything the hook leaves alone.
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

// v1alpha2 has no resourcesTemplate. A v1alpha1 template that carried a Helm string comes up without
// it and marked with the legacy-helm-template annotation, so the controller refuses to render the
// empty structured shape over the release that string built; a template whose string was empty (or
// whitespace) is not marked -- there was nothing to lose.
func TestProjectTemplateUpConversionDropsTheHelmString(t *testing.T) {
	t.Parallel()

	const mark = "projects.deckhouse.io/legacy-helm-template"

	tests := []struct {
		name     string
		resource any
		marked   bool
	}{
		{name: "a helm string", resource: "---\napiVersion: v1\nkind: Namespace\n", marked: true},
		{name: "an empty string", resource: "", marked: false},
		{name: "whitespace only", resource: "  \n\t", marked: false},
		{name: "no field at all", resource: nil, marked: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := map[string]any{
				"description":      "a template",
				"parametersSchema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
			}
			if tt.resource != nil {
				spec["resourcesTemplate"] = tt.resource
			}
			template := map[string]any{
				"apiVersion": "deckhouse.io/v1alpha1",
				"kind":       "ProjectTemplate",
				"metadata":   map[string]any{"name": "test", "annotations": map[string]any{"keep": "me"}},
				"spec":       spec,
			}

			up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", template)
			assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
			assert.Nil(t, specField(up, "resourcesTemplate"), "v1alpha2 must not carry the Helm string")
			assert.Equal(t, "a template", specField(up, "description"))
			assert.Equal(t, spec["parametersSchema"], specField(up, "parametersSchema"))

			annotations, _ := up["metadata"].(map[string]any)["annotations"].(map[string]any)
			assert.Equal(t, "me", annotations["keep"], "the other annotations survive")
			if tt.marked {
				assert.Equal(t, "true", annotations[mark])
				assert.Equal(t, tt.resource, annotations[mark+"-body"],
					"the string the administrator is asked to rewrite must stay readable")
			} else {
				assert.NotContains(t, annotations, mark)
				assert.NotContains(t, annotations, mark+"-body")
			}
		})
	}
}

// A Helm string too large to live in an annotation still gets the mark: the projects must be parked
// either way. All annotations of an object together may not exceed 256 KiB, and an object carrying a
// body near that cap could not be written back -- which is the very thing the administrator has to
// do to fix it. The limit is 64 KiB of UTF-8, counted in bytes and not in characters.
func TestProjectTemplateUpConversionSkipsAnOversizedHelmString(t *testing.T) {
	t.Parallel()

	const (
		mark     = "projects.deckhouse.io/legacy-helm-template"
		limit    = 64 << 10
		twoBytes = "é" // two bytes in UTF-8
	)

	tests := []struct {
		name     string
		resource string
		hasBody  bool
	}{
		{name: "exactly the limit", resource: strings.Repeat("x", limit), hasBody: true},
		{name: "one byte over the limit", resource: strings.Repeat("x", limit+1), hasBody: false},
		{name: "the limit in two-byte characters", resource: strings.Repeat(twoBytes, limit/2), hasBody: true},
		{name: "fewer characters than the limit but more bytes", resource: strings.Repeat(twoBytes, limit/2+1), hasBody: false},
	}

	for _, engine := range jqEngines(t) {
		for _, tt := range tests {
			t.Run(engine.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				template := map[string]any{
					"apiVersion": "deckhouse.io/v1alpha1",
					"kind":       "ProjectTemplate",
					"metadata":   map[string]any{"name": "test"},
					"spec":       map[string]any{"resourcesTemplate": tt.resource},
				}

				converted := runConversion(t, engine, templateConversionHook, "v1alpha1_to_v1alpha2", []any{template})
				up, ok := converted[0].(map[string]any)
				require.True(t, ok, "the conversion produced %v", converted[0])

				metadata, _ := up["metadata"].(map[string]any)
				annotations, _ := metadata["annotations"].(map[string]any)
				assert.Equal(t, "true", annotations[mark], "the projects must be parked either way")
				if tt.hasBody {
					assert.Equal(t, tt.resource, annotations[mark+"-body"])
				} else {
					assert.NotContains(t, annotations, mark+"-body")
				}
				assert.Nil(t, specField(up, "resourcesTemplate"))
			})
		}
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

// A round trip through v1alpha1 keeps what both versions can describe. The Helm string is not among
// that: it goes down as the empty string v1alpha1 requires and does not come back.
func TestProjectTemplateVersionBump(t *testing.T) {
	t.Parallel()

	template := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha2",
		"kind":       "ProjectTemplate",
		"metadata":   map[string]any{"name": "test"},
		"spec": map[string]any{
			"description":      "a template",
			"parametersSchema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
		},
	}

	down := convertWith(t, templateConversionHook, "v1alpha2_to_v1alpha1", template)
	assert.Equal(t, "deckhouse.io/v1alpha1", down["apiVersion"])
	assert.Equal(t, specField(template, "description"), specField(down, "description"))
	assert.Equal(t, "", specField(down, "resourcesTemplate"))

	up := convertWith(t, templateConversionHook, "v1alpha1_to_v1alpha2", down)
	assert.Equal(t, "deckhouse.io/v1alpha2", up["apiVersion"])
	assert.Equal(t, template["spec"], up["spec"])
	annotations, _ := up["metadata"].(map[string]any)["annotations"].(map[string]any)
	assert.NotContains(t, annotations, "projects.deckhouse.io/legacy-helm-template", "an empty string is not a Helm template")
}

// A structured template has neither of the two fields the v1alpha1 schema requires, and the apiserver
// validates what a conversion returns, so they are backfilled rather than left missing.
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
