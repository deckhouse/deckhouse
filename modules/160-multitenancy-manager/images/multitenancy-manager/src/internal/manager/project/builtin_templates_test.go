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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
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
			// the required-requests policy is rendered only for a declared parameter, so these
			// templates declare it and keep the policy on unless the project turns it off
			required, ok := props["requiredRequests"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, true, required["default"])
		})
	}
}

// helmTemplateFixtures holds, per built-in template, a project as its former Helm text rendered it
// (project.yaml), what that text rendered (resources.yaml) and the project the conversion makes of it
// (converted.yaml). TestNativeRenderSelectsWhatTheHelmTemplateSelected in the helm package shows that
// the structured built-in renders everything else under the same names.
const helmTemplateFixtures = "../../helm/testdata/helm_template"

// When a built-in template becomes structured, the upgrade of a release its former Helm text rendered
// drops the AuthorizationRule of every administrator and the all-pods quota, and the standard fields
// of the converted project replace exactly these: the handover keeps them until d8-administrators and
// d8-project-quota work. Nothing else of the render is replaced.
func TestBuiltinHelmTextsAreHandedOver(t *testing.T) {
	for _, template := range []string{"default", "secure", "secure-with-dedicated-nodes"} {
		t.Run(template, func(t *testing.T) {
			base := filepath.Join(helmTemplateFixtures, template)
			raw, err := os.ReadFile(filepath.Join(base, "converted.yaml"))
			require.NoError(t, err)
			project := new(v1alpha3.Project)
			require.NoError(t, yaml.Unmarshal(raw, project))
			raw, err = os.ReadFile(filepath.Join(base, "project.yaml"))
			require.NoError(t, err)
			rendered := new(struct {
				Spec struct {
					Parameters map[string]any `json:"parameters"`
				} `json:"spec"`
			})
			require.NoError(t, yaml.Unmarshal(raw, rendered))
			deployed := &helm.ProjectRelease{HelmText: true, Administrators: helm.ParameterAdministrators(rendered.Spec.Parameters)}
			require.NotEmpty(t, deployed.Administrators)
			administrators := handoverAdministrators(project, deployed)

			raw, err = os.ReadFile(filepath.Join(base, "resources.yaml"))
			require.NoError(t, err)
			replaced := map[string]bool{}
			for _, doc := range releaseutil.SplitManifests(string(raw)) {
				live := new(unstructured.Unstructured)
				require.NoError(t, yaml.Unmarshal([]byte(doc), &live.Object))
				if live.GetKind() == "" {
					continue
				}
				// Helm records the release on every object it applies.
				annotations := live.GetAnnotations()
				if annotations == nil {
					annotations = map[string]string{}
				}
				annotations[helm.ResourceAnnotationReleaseName] = helm.ReleaseName(project.Name)
				live.SetAnnotations(annotations)
				key := keyOf(releaseObjectOf(live), project.Name)
				replaced[live.GetKind()+"/"+live.GetName()] = replacedByStandardFields(project, key, live, administrators)
			}

			rules := 0
			for name, ok := range replaced {
				kind, _, _ := strings.Cut(name, "/")
				if kind == "AuthorizationRule" {
					rules++
				}
				assert.Equal(t, kind == "AuthorizationRule" || name == "ResourceQuota/all-pods", ok, name)
			}
			assert.Equal(t, len(project.Spec.Administrators), rules, "a rule per administrator")
			assert.Contains(t, replaced, "ResourceQuota/all-pods")
		})
	}
}
