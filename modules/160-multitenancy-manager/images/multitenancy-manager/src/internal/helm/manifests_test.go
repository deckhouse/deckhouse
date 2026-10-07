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

package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
)

// manifestsSchema declares a free-form string parameter with a default, and the administrators and
// the quota the way a text written for the v1alpha2 Project layout declares them.
var manifestsSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"team":           map[string]any{"type": "string", "default": "platform"},
		"administrators": map[string]any{"type": "array"},
		"resourceQuota": map[string]any{"type": "object", "properties": map[string]any{
			"requests": map[string]any{"type": "object", "properties": map[string]any{"cpu": map[string]any{"type": "string"}}},
		}},
	},
}

func manifestsProject(parameters map[string]any) *v1alpha3.Project {
	return &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj"},
		Spec: v1alpha3.ProjectSpec{
			ProjectTemplateName: "custom",
			Administrators:      []v1alpha3.Administrator{{Kind: "User", Name: "alice"}},
			Quota:               corev1.ResourceList{"requests.cpu": resource.MustParse("2")},
			Parameters:          parameters,
		},
	}
}

// renderedObjects reads the rendered manifests into their objects, without the empty documents.
func renderedObjects(t *testing.T, rendered string) []*unstructured.Unstructured {
	t.Helper()
	var objects []*unstructured.Unstructured
	for _, manifest := range releaseutil.SplitManifests(rendered) {
		object := new(unstructured.Unstructured)
		require.NoError(t, yaml.Unmarshal([]byte(manifest), object))
		if object.GetKind() != "" {
			objects = append(objects, object)
		}
	}
	return objects
}

// The manifests text gets what a resourcesTemplate gets: the project name, the template name, the
// parameters completed with the defaults of the schema, and the administrators and the quota where
// the schema declares them, with the helpers of the chart.
func TestRenderManifestsWithTheValuesOfAHelmText(t *testing.T) {
	client := injectionTestClient(t)
	const manifests = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "normalize" .projectName }}-settings
data:
  template: {{ .projectTemplateName | quote }}
  team: {{ .parameters.team | quote }}
  admin: {{ (index .parameters.administrators 0).name | quote }}
  cpu: {{ .parameters.resourceQuota.requests.cpu | quote }}
`

	rendered, err := client.RenderManifests(manifestsProject(nil), manifests, manifestsSchema)
	require.NoError(t, err)

	objects := renderedObjects(t, rendered)
	require.Len(t, objects, 1)
	assert.Equal(t, "proj-settings", objects[0].GetName())
	data, _, err := unstructured.NestedStringMap(objects[0].Object, "data")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"template": "custom", "team": "platform", "admin": "alice", "cpu": "2"}, data)
}

// A parameter that becomes structure instead of a value is refused as in a resourcesTemplate. A line
// break inside a quoted value passes.
func TestRenderManifestsRefusesParameterInjection(t *testing.T) {
	client := injectionTestClient(t)
	const unquoted = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: {{ .parameters.team }}
`

	_, err := client.RenderManifests(manifestsProject(map[string]any{"team": clusterAdminPayload}), unquoted, manifestsSchema)
	require.ErrorIs(t, err, ErrParameterInjection, "a parameter that adds an object")

	const quoted = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  team: {{ .parameters.team | quote }}
`
	rendered, err := client.RenderManifests(manifestsProject(map[string]any{"team": "line\nbreak"}), quoted, manifestsSchema)
	require.NoError(t, err)
	assert.Len(t, renderedObjects(t, rendered), 1)
}

// An administrator name reaches the text through the parameters, and is checked like any other value.
func TestRenderManifestsRefusesAnInjectedAdministrator(t *testing.T) {
	client := injectionTestClient(t)
	project := manifestsProject(nil)
	project.Spec.Administrators = []v1alpha3.Administrator{{Kind: "User", Name: "alice --- kind: Secret"}}

	_, err := client.RenderManifests(project, unquotedAdministrators, manifestsSchema)
	require.ErrorIs(t, err, ErrParameterInjection)
}

// The render has no cluster to read from: lookup finds nothing, so a text cannot copy into a project
// what the user of the project cannot read.
func TestRenderManifestsLooksUpNothing(t *testing.T) {
	client := injectionTestClient(t)
	const manifests = `
{{- $secret := lookup "v1" "Secret" "d8-system" "deckhouse-registry" }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: copied
data:
  found: {{ empty $secret | ternary "nothing" "something" | quote }}
`

	rendered, err := client.RenderManifests(manifestsProject(nil), manifests, manifestsSchema)
	require.NoError(t, err)
	objects := renderedObjects(t, rendered)
	require.Len(t, objects, 1)
	found, _, err := unstructured.NestedString(objects[0].Object, "data", "found")
	require.NoError(t, err)
	assert.Equal(t, "nothing", found)
}

// A text that does not render is an error of the render, not an empty set of objects.
func TestRenderManifestsReportsATextThatDoesNotRender(t *testing.T) {
	client := injectionTestClient(t)

	_, err := client.RenderManifests(manifestsProject(nil), "{{ .parameters.team | nosuchfunction }}", manifestsSchema)
	require.Error(t, err)
}
