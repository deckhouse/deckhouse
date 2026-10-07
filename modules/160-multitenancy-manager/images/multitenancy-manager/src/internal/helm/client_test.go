/*
Copyright 2024 Flant JSC

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
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/validate"
)

func parseManifest(raw string) (*unstructured.Unstructured, bool, error) {
	object := new(unstructured.Unstructured)
	if err := yaml.Unmarshal([]byte(raw), object); err != nil {
		return nil, false, err
	}
	if object.GetAPIVersion() == "" || object.GetKind() == "" {
		return nil, false, nil
	}

	// Normalize fields that can differ between serializers/versions.
	unstructured.RemoveNestedField(object.Object, "metadata", "creationTimestamp")

	return object, true, nil
}

// Test renders the Helm text of each case through the chart and the post-renderer and compares the
// objects with the golden resources.yaml of the case. No built-in template is a Helm text: they are
// structured, and TestNativeRender compares their native render with the golden files of their cases.
//
// simple_case holds the Helm text the built-in simple template had before it became structured, and
// TestNativeRender renders the shipped simple.yaml against the same golden: the two paths have to
// produce one set of objects, the template-labels record on the main namespace included.
func Test(t *testing.T) {
	templates, err := parseHelmTemplates("../../helmlib")
	assert.Nil(t, err)
	for _, c := range []string{"simple_case", "empty_case", "without_ns_case", "skip_heritage_and_unmanaged_case"} {
		t.Run(c, func(t *testing.T) {
			basePath := filepath.Join("./testdata", c)
			assert.Nil(t, test(templates, basePath, filepath.Join(basePath, "template.yaml")))
		})
	}
}

func test(templates map[string][]byte, basePath, templatePath string) error {
	projectTemplate, err := read[v1alpha1.ProjectTemplate](templatePath)
	if err != nil {
		return err
	}

	// validation reads only parametersSchema, which v1alpha2 shares with v1alpha1
	schemaTemplate, err := read[v1alpha2.ProjectTemplate](templatePath)
	if err != nil {
		return err
	}
	if err = validate.ProjectTemplate(schemaTemplate); err != nil {
		return err
	}

	project, err := read[v1alpha3.Project](filepath.Join(basePath, "project.yaml"))
	if err != nil {
		return err
	}

	if err = validate.Project(project, schemaTemplate); err != nil {
		return err
	}

	// Use isFirstInstall=true for standard tests to include unmanaged resources
	buf, err := render(templates, project, projectTemplate, true)
	if err != nil {
		return err
	}
	rendered := releaseutil.SplitManifests(buf.String())

	// uncomment for test and render rendered resources
	if os.Getenv("REGEN_GOLDEN") != "" {
		os.WriteFile(filepath.Join(basePath, "resources.yaml"), buf.Bytes(), 0644)
	}

	rawExpected, err := os.ReadFile(filepath.Join(basePath, "resources.yaml"))
	if err != nil {
		return err
	}
	expected := releaseutil.SplitManifests(string(rawExpected))

	renderedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range rendered {
		object, ok, parseErr := parseManifest(raw)
		if parseErr != nil {
			return parseErr
		}
		if !ok {
			continue
		}
		renderedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = object
	}

	expectedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range expected {
		object, ok, parseErr := parseManifest(raw)
		if parseErr != nil {
			return parseErr
		}
		if !ok {
			continue
		}
		expectedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = object
	}

	for name := range renderedMap {
		if _, ok := expectedMap[name]; !ok {
			return fmt.Errorf("rendered manifests don't match the expected manifests: resource '%s' not found", name)
		}
		if diff := cmp.Diff(renderedMap[name].Object, expectedMap[name].Object, cmpopts.EquateEmpty()); diff != "" {
			fmt.Println(diff)
			return fmt.Errorf("rendered manifest '%s' doesn't match the expected manifest: %s", name, diff)
		}
	}

	for name := range expectedMap {
		if _, ok := renderedMap[name]; !ok {
			return fmt.Errorf("expected manifests don't match the rendered manifests: resource '%s' not found", name)
		}
		if diff := cmp.Diff(renderedMap[name].Object, expectedMap[name].Object, cmpopts.EquateEmpty()); diff != "" {
			fmt.Println(diff)
			return fmt.Errorf("expected '%s' manifest doesn't match the rendered manifests: %s", name, diff)
		}
	}

	return nil
}

func TestUnmanagedResourcesFirstInstall(t *testing.T) {
	templates, err := parseHelmTemplates("../../helmlib")
	assert.Nil(t, err)

	basePath := filepath.Join("./testdata", "skip_heritage_and_unmanaged_case")
	projectTemplate, err := read[v1alpha1.ProjectTemplate](filepath.Join(basePath, "template.yaml"))
	assert.Nil(t, err)

	project, err := read[v1alpha3.Project](filepath.Join(basePath, "project.yaml"))
	assert.Nil(t, err)

	// Test first install - unmanaged resources should be included
	buf, err := render(templates, project, projectTemplate, true)
	assert.Nil(t, err)

	rendered := releaseutil.SplitManifests(buf.String())
	renderedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range rendered {
		object, ok, parseErr := parseManifest(raw)
		if parseErr != nil {
			t.Fatalf("failed to unmarshal: %v", parseErr)
		}
		if !ok {
			continue
		}
		renderedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = object
	}

	// Check that unmanaged resource is present
	unmanagedKey := "v1.ConfigMap.test.unmanaged"
	if _, ok := renderedMap[unmanagedKey]; !ok {
		t.Errorf("unmanaged resource should be present on first install, but it's missing")
	}

	// Verify unmanaged resource has correct annotations and labels
	unmanagedObj := renderedMap[unmanagedKey]

	annotations := unmanagedObj.GetAnnotations()
	if annotations["helm.sh/resource-policy"] != "keep" {
		t.Errorf("unmanaged resource should have helm.sh/resource-policy=keep annotation, got: %v", annotations)
	}

	labels := unmanagedObj.GetLabels()
	if labels[v1alpha3.ResourceLabelHeritage] != "" {
		t.Errorf("unmanaged resource should not have heritage label, but got: %s", labels[v1alpha3.ResourceLabelHeritage])
	}
	if labels[v1alpha3.ResourceLabelProject] != project.Name {
		t.Errorf("unmanaged resource should have project label, got: %s", labels[v1alpha3.ResourceLabelProject])
	}

	// Compare with expected first install resources
	rawExpected, err := os.ReadFile(filepath.Join(basePath, "resources_first_install.yaml"))
	assert.Nil(t, err)
	expected := releaseutil.SplitManifests(string(rawExpected))

	expectedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range expected {
		object, ok, parseErr := parseManifest(raw)
		if parseErr != nil {
			t.Fatalf("failed to unmarshal expected: %v", parseErr)
		}
		if !ok {
			continue
		}
		expectedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = object
	}

	for name := range renderedMap {
		if _, ok := expectedMap[name]; !ok {
			t.Errorf("rendered resource '%s' not found in expected first install manifests", name)
		} else if diff := cmp.Diff(renderedMap[name].Object, expectedMap[name].Object, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("rendered manifest '%s' doesn't match expected first install manifest: %s", name, diff)
		}
	}

	for name := range expectedMap {
		if _, ok := renderedMap[name]; !ok {
			t.Errorf("expected first install resource '%s' not found in rendered manifests", name)
		}
	}
}

func TestUnmanagedResourcesUpgrade(t *testing.T) {
	templates, err := parseHelmTemplates("../../helmlib")
	assert.Nil(t, err)

	basePath := filepath.Join("./testdata", "skip_heritage_and_unmanaged_case")
	projectTemplate, err := read[v1alpha1.ProjectTemplate](filepath.Join(basePath, "template.yaml"))
	assert.Nil(t, err)

	project, err := read[v1alpha3.Project](filepath.Join(basePath, "project.yaml"))
	assert.Nil(t, err)

	// Test upgrade - unmanaged resources should be excluded
	buf, err := render(templates, project, projectTemplate, false)
	assert.Nil(t, err)

	rendered := releaseutil.SplitManifests(buf.String())
	renderedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range rendered {
		object, ok, parseErr := parseManifest(raw)
		if parseErr != nil {
			t.Fatalf("failed to unmarshal: %v", parseErr)
		}
		if !ok {
			continue
		}
		renderedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = object
	}

	// Check that unmanaged resource is NOT present
	unmanagedKey := "v1.ConfigMap.test.unmanaged"
	if _, ok := renderedMap[unmanagedKey]; ok {
		t.Errorf("unmanaged resource should NOT be present on upgrade, but it was found")
	}

	// Compare with expected upgrade resources (without unmanaged)
	rawExpected, err := os.ReadFile(filepath.Join(basePath, "resources_upgrade.yaml"))
	assert.Nil(t, err)
	expected := releaseutil.SplitManifests(string(rawExpected))

	expectedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range expected {
		object, ok, parseErr := parseManifest(raw)
		if parseErr != nil {
			t.Fatalf("failed to unmarshal expected: %v", parseErr)
		}
		if !ok {
			continue
		}
		expectedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = object
	}

	for name := range renderedMap {
		if _, ok := expectedMap[name]; !ok {
			t.Errorf("rendered resource '%s' not found in expected upgrade manifests", name)
		} else if diff := cmp.Diff(renderedMap[name].Object, expectedMap[name].Object, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("rendered manifest '%s' doesn't match expected upgrade manifest: %s", name, diff)
		}
	}

	for name := range expectedMap {
		if _, ok := renderedMap[name]; !ok {
			t.Errorf("expected upgrade resource '%s' not found in rendered manifests", name)
		}
	}
}

// A Helm text written for the v1alpha2 Project layout reads the administrators and the quota from
// .parameters. The conversion moves both into spec.administrators and spec.quota, and the text still
// gets them where it reads them: a project in the v1alpha3 layout renders the objects the same project
// rendered in the v1alpha2 layout, which is what release 1.77 rendered from it. The quota values are
// in their canonical spelling, the only one storage holds once the controller has written the project,
// unless the schema pins a unit, and a text renders the spelling it is given.
func TestHelmTextsOfTheParametersLayoutRenderAsBefore(t *testing.T) {
	t.Parallel()

	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)

	// what the default template of that layout takes, in both layouts
	administrators := []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}, {Kind: "Group", Name: "team"}}
	quota := corev1.ResourceList{
		"requests.cpu":     resource.MustParse("2"),
		"requests.memory":  resource.MustParse("4Gi"),
		"requests.storage": resource.MustParse("10Gi"),
		"limits.cpu":       resource.MustParse("4"),
		"limits.memory":    resource.MustParse("8Gi"),
	}
	inParameters := map[string]any{
		"administrators": []any{
			map[string]any{"subject": "User", "name": "alice@example.com"},
			map[string]any{"subject": "Group", "name": "team"},
		},
		"resourceQuota": map[string]any{
			// a whole number the parameter held as a JSON number
			"requests": map[string]any{"cpu": int64(2), "memory": "4Gi", "storage": "10Gi"},
			"limits":   map[string]any{"cpu": "4", "memory": "8Gi"},
		},
	}
	builtinParameters := map[string]any{
		"networkPolicy":             "Isolated",
		"podSecurityProfile":        "Restricted",
		"extendedMonitoringEnabled": true,
		"clusterLogDestinationName": "loki",
	}

	tests := []struct {
		name     string
		template string
		// quotaSchema replaces the resourceQuota schema of the template when it is set
		quotaSchema string
		// parameters are the ones the text reads besides the two standard fields
		parameters map[string]any
		// inParameters are the two standard fields as the v1alpha2 layout held them in the parameters
		inParameters   map[string]any
		administrators []v1alpha3.Administrator
		quota          corev1.ResourceList
		// rules is the number of AuthorizationRules the text renders
		rules int
	}{
		{name: "default", template: "default", parameters: builtinParameters, inParameters: inParameters, administrators: administrators, quota: quota, rules: 2},
		{name: "secure", template: "secure", parameters: builtinParameters, inParameters: inParameters, administrators: administrators, quota: quota, rules: 2},
		{
			// the typed quota spells 2000m as 2 and 4096Mi as 4Gi, which the patterns refuse
			name:     "default with a schema that pins the units",
			template: "default",
			quotaSchema: `
type: object
properties:
  requests:
    type: object
    properties:
      cpu: {type: string, pattern: "^[0-9]+m$"}
      memory: {type: string, pattern: "^[0-9]+Mi$"}`,
			parameters: builtinParameters,
			inParameters: map[string]any{
				"administrators": inParameters["administrators"],
				"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "2000m", "memory": "4096Mi"}},
			},
			administrators: administrators,
			quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("2000m"), "requests.memory": resource.MustParse("4096Mi")},
			rules:          2,
		},
		{
			name:     "secure-with-dedicated-nodes",
			template: "secure-with-dedicated-nodes",
			parameters: map[string]any{
				"networkPolicy":       "Isolated",
				"podSecurityProfile":  "Restricted",
				"allowedUIDs":         map[string]any{"min": int64(1000), "max": int64(2000)},
				"runtimeAuditEnabled": true,
				"dedicatedNodes":      map[string]any{"nodeSelector": map[string]any{"node-role.kubernetes.io/project": "team"}},
			},
			inParameters:   inParameters,
			administrators: administrators,
			quota:          quota,
			rules:          2,
		},
		{
			name:     "roles-from-a-parameter",
			template: "roles-from-a-parameter",
			parameters: map[string]any{
				"roleBindings": []any{
					map[string]any{"name": "admins", "accessLevel": "Admin", "subjects": []any{map[string]any{"kind": "Group", "name": "team"}}},
					map[string]any{"name": "viewers", "accessLevel": "User", "subjects": []any{map[string]any{"kind": "User", "name": "bob@example.com"}}},
				},
			},
			inParameters: map[string]any{
				"resourceQuota": map[string]any{"requests": map[string]any{"cpu": int64(2), "memory": "4Gi"}},
			},
			quota: corev1.ResourceList{"requests.cpu": resource.MustParse("2"), "requests.memory": resource.MustParse("4Gi")},
			rules: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", "compat177", tt.template+".yaml")
			template, err := read[v1alpha1.ProjectTemplate](path)
			require.NoError(t, err)
			schemaTemplate, err := read[v1alpha2.ProjectTemplate](path)
			require.NoError(t, err)
			if tt.quotaSchema != "" {
				for _, schema := range []map[string]any{template.Spec.ParametersSchema.OpenAPIV3Schema, schemaTemplate.Spec.ParametersSchema.OpenAPIV3Schema} {
					quotaSchema := map[string]any{}
					require.NoError(t, yaml.Unmarshal([]byte(tt.quotaSchema), &quotaSchema))
					properties, ok := schema["properties"].(map[string]any)
					require.True(t, ok, "the schema of %s has properties", tt.template)
					properties["resourceQuota"] = quotaSchema
				}
			}

			inLayout := &v1alpha3.Project{}
			inLayout.Name = "team"
			inLayout.Spec.ProjectTemplateName = "custom"
			inLayout.Spec.Parameters = maps.Clone(tt.parameters)
			maps.Copy(inLayout.Spec.Parameters, tt.inParameters)

			lifted := inLayout.DeepCopy()
			lifted.Spec.Parameters = maps.Clone(tt.parameters)
			lifted.Spec.Administrators = tt.administrators
			lifted.Spec.Quota = tt.quota

			objects := func(project *v1alpha3.Project) []string {
				t.Helper()
				require.NoError(t, validate.Project(project, schemaTemplate))
				buf, err := render(templates, project.DeepCopy(), template, true)
				require.NoError(t, err)
				canonical, err := canonicalObjects(buf.String())
				require.NoError(t, err)
				return slices.Sorted(maps.Keys(canonical))
			}

			before, after := objects(inLayout), objects(lifted)
			assert.Equal(t, before, after)
			assert.Len(t, slices.DeleteFunc(slices.Clone(after), func(object string) bool {
				return !strings.Contains(object, "kind: AuthorizationRule")
			}), tt.rules)
			assert.True(t, slices.ContainsFunc(after, func(object string) bool {
				return strings.Contains(object, "kind: ResourceQuota") && strings.Contains(object, "requests.cpu")
			}), "the quota is rendered from spec.quota")
		})
	}
}

// The conversion leaves administrators or resourceQuota in spec.parameters when it cannot give them back
// as they were, and validation leaves out only one the schema does not declare. A Helm text whose
// schema declares them still reads them from .parameters: a project as the conversion leaves it renders
// the objects the same project rendered in the v1alpha2 layout. The text skips the number 0 with
// "with", so a quota that holds it renders no key for it, as in release 1.77.
func TestHelmTextsGetWhatTheConversionLeavesInTheParameters(t *testing.T) {
	t.Parallel()

	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)
	path := filepath.Join("testdata", "compat177", "default.yaml")
	template, err := read[v1alpha1.ProjectTemplate](path)
	require.NoError(t, err)
	schemaTemplate, err := read[v1alpha2.ProjectTemplate](path)
	require.NoError(t, err)

	builtinParameters := map[string]any{"networkPolicy": "Isolated", "podSecurityProfile": "Baseline"}
	zeroQuota := map[string]any{"requests": map[string]any{"cpu": int64(0), "memory": "2Gi"}}
	tests := []struct {
		name string
		// inLayout are the parameters of the project in the v1alpha2 layout
		inLayout map[string]any
		// left is the spec the conversion makes of it
		left v1alpha3.ProjectSpec
		// rules is the number of AuthorizationRules the text renders
		rules int
		// hard and notHard are quota keys the text does and does not render
		hard, notHard string
	}{
		{
			name: "a quota with the number 0",
			inLayout: map[string]any{
				"administrators": []any{map[string]any{"subject": "User", "name": "alice@example.com"}},
				"resourceQuota":  zeroQuota,
			},
			left: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice@example.com"}},
				Parameters:     map[string]any{"resourceQuota": zeroQuota},
			},
			rules:   1,
			hard:    "requests.memory",
			notHard: "requests.cpu",
		},
		{
			name: "an empty administrators list",
			inLayout: map[string]any{
				"administrators": []any{},
				"resourceQuota":  map[string]any{"requests": map[string]any{"cpu": "1"}},
			},
			left: v1alpha3.ProjectSpec{
				Quota:      corev1.ResourceList{"requests.cpu": resource.MustParse("1")},
				Parameters: map[string]any{"administrators": []any{}},
			},
			hard:    "requests.cpu",
			notHard: "requests.memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := func(spec v1alpha3.ProjectSpec) []string {
				t.Helper()
				project := &v1alpha3.Project{Spec: *spec.DeepCopy()}
				project.Name = "team"
				project.Spec.ProjectTemplateName = "custom"
				maps.Copy(project.Spec.Parameters, builtinParameters)
				require.NoError(t, validate.Project(project, schemaTemplate))
				buf, err := render(templates, project, template, true)
				require.NoError(t, err)
				canonical, err := canonicalObjects(buf.String())
				require.NoError(t, err)
				return slices.Sorted(maps.Keys(canonical))
			}

			before := objects(v1alpha3.ProjectSpec{Parameters: tt.inLayout})
			after := objects(tt.left)
			assert.Equal(t, before, after)
			assert.Len(t, slices.DeleteFunc(slices.Clone(after), func(object string) bool {
				return !strings.Contains(object, "kind: AuthorizationRule")
			}), tt.rules)
			quota := slices.IndexFunc(after, func(object string) bool {
				return strings.Contains(object, "kind: ResourceQuota") && strings.Contains(object, "name: all-pods")
			})
			require.GreaterOrEqual(t, quota, 0, "the text renders its quota")
			assert.Contains(t, after[quota], tt.hard)
			assert.NotContains(t, after[quota], tt.notHard)
		})
	}
}

// TestListItemsArePostRendered guards the List expansion: Helm flattens a kind: List only after the
// post-renderer, so every item has to get the project labels and the project namespace here, or it
// would be applied without them. A ResourceQuota inside is an object like any other.
func TestListItemsArePostRendered(t *testing.T) {
	t.Parallel()

	manifest := `
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: all-pods
    spec:
      hard:
        pods: "10"
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: elsewhere
      namespace: another-project
    data:
      a: b
`
	project := &v1alpha3.Project{}
	project.Name = "test"
	project.Spec.ProjectTemplateName = "custom"

	post := newPostRenderer(project, nil, ctrl.Log.WithName("test"), true)
	out, err := post.Run(bytes.NewBufferString(manifest))
	require.NoError(t, err)
	assert.ErrorIs(t, post.warning, ErrNamespaceOverride, "an item in a foreign namespace is moved, with a warning")

	rendered := make(map[string]*unstructured.Unstructured)
	for _, raw := range releaseutil.SplitManifests(out.String()) {
		object, ok, parseErr := parseManifest(raw)
		require.NoError(t, parseErr)
		if ok {
			rendered[object.GetKind()+"."+object.GetName()] = object
		}
	}

	for _, key := range []string{"ResourceQuota.all-pods", "ConfigMap.elsewhere"} {
		object, ok := rendered[key]
		require.True(t, ok, "%s is rendered on its own", key)
		assert.Equal(t, "test", object.GetNamespace(), key)
		assert.Equal(t, map[string]string{
			v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
			v1alpha3.ResourceLabelProject:  "test",
			v1alpha3.ResourceLabelTemplate: "custom",
		}, object.GetLabels(), key)
	}
	assert.NotContains(t, rendered, "List.", "the wrapper itself is not rendered")
}

// The main namespace of a Helm text carries the template-labels record: the labels the text put on
// it, without the module-owned ones and the GitOps tracking ones, and never a record the text wrote
// itself. Other namespaces are dropped, and a natively rendered namespace keeps the record the native
// render wrote.
func TestHelmMainNamespaceRecordsTemplateLabels(t *testing.T) {
	t.Parallel()

	const manifest = `
apiVersion: v1
kind: Namespace
metadata:
  name: test
  labels:
    team: backend
    security.deckhouse.io/pod-policy: baseline
    argocd.argoproj.io/instance: platform
  annotations:
    projects.deckhouse.io/template-labels: '{"forged":"yes"}'
`
	record := func(t *testing.T, post *postRenderer, source string) string {
		t.Helper()
		out, err := post.Run(bytes.NewBufferString(source))
		require.NoError(t, err)
		object, ok, err := parseManifest(releaseutil.SplitManifests(out.String())["manifest-0"])
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "Namespace", object.GetKind())
		return object.GetAnnotations()["projects.deckhouse.io/template-labels"]
	}
	project := func() *v1alpha3.Project {
		project := &v1alpha3.Project{}
		project.Name = "test"
		return project
	}

	t.Run("a Helm text", func(t *testing.T) {
		post := newPostRenderer(project(), nil, ctrl.Log.WithName("test"), true)
		assert.JSONEq(t, `{"team":"backend"}`, record(t, post, manifest))
	})

	t.Run("a Helm text that renders no namespace", func(t *testing.T) {
		post := newPostRenderer(project(), nil, ctrl.Log.WithName("test"), true)
		assert.JSONEq(t, `{}`, record(t, post, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"))
	})

	t.Run("a native render", func(t *testing.T) {
		post := newPostRenderer(project(), nil, ctrl.Log.WithName("test"), true)
		post.manifests = "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n  labels:\n    team: backend\n" +
			"  annotations:\n    projects.deckhouse.io/template-labels: '{\"from\":\"the native render\"}'\n"
		assert.JSONEq(t, `{"from":"the native render"}`, record(t, post, ""))
	})
}

// A release a previous version applied from the same text and values has the hash of the chart and
// the values alone. It must not pass for up to date, or its main namespace would never get the
// template-labels record the post-renderer adds now.
func TestHelmTextHashCoversThePostRenderer(t *testing.T) {
	t.Parallel()

	templates, err := parseHelmTemplates("../../helmlib")
	require.NoError(t, err)
	project := &v1alpha3.Project{}
	project.Name = "test"
	template := caseTemplate(t, "simple")
	values := buildValues(project, template, templateParameters(project, template))

	assert.NotEqual(t, hashMD5(templates, values), helmTextHash(templates, values))
	assert.Equal(t, helmTextHash(templates, values), helmTextHash(templates, buildValues(project, template, templateParameters(project, template))))
	assert.NotContains(t, values, "postRendererRevision", "the values the chart renders are left alone")
}

// TestCollectRoleRefs checks that the post-renderer extracts the roleRef of every binding kind
// (ProjectRoleBinding/ClusterProjectRoleBinding via spec.roleRef, native RoleBinding/ClusterRoleBinding
// via roleRef) and ignores non-binding objects.
func TestCollectRoleRefs(t *testing.T) {
	manifest := `
apiVersion: v1
kind: List
items:
  - apiVersion: deckhouse.io/v1alpha3
    kind: ProjectRoleBinding
    metadata:
      name: prb-disabled
    spec:
      roleRef:
        kind: ClusterRole
        name: d8:project:secret-reader
  - apiVersion: deckhouse.io/v1alpha3
    kind: ClusterProjectRoleBinding
    metadata:
      name: cprb-admin
    spec:
      roleRef:
        kind: ClusterRole
        name: d8:project:admin
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: rb-clusterrole
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: view
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: rb-role
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: Role
      name: some-role
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: not-a-binding
    data:
      a: b
`
	project := &v1alpha3.Project{}
	project.Name = "test"

	post := newPostRenderer(project, nil, ctrl.Log.WithName("test"), true)
	_, err := post.Run(bytes.NewBufferString(manifest))
	assert.Nil(t, err)

	refs := make(map[string]BindingRoleRef)
	for _, ref := range post.referencedRoles {
		refs[ref.BindingName] = ref
	}

	assert.Len(t, refs, 4, "the ConfigMap must not be collected as a binding")
	assert.Equal(t, BindingRoleRef{BindingKind: "ProjectRoleBinding", BindingName: "prb-disabled", RoleKind: "ClusterRole", RoleName: "d8:project:secret-reader"}, refs["prb-disabled"])
	assert.Equal(t, BindingRoleRef{BindingKind: "ClusterProjectRoleBinding", BindingName: "cprb-admin", RoleKind: "ClusterRole", RoleName: "d8:project:admin"}, refs["cprb-admin"])
	assert.Equal(t, BindingRoleRef{BindingKind: "RoleBinding", BindingName: "rb-clusterrole", RoleKind: "ClusterRole", RoleName: "view"}, refs["rb-clusterrole"])
	assert.Equal(t, BindingRoleRef{BindingKind: "RoleBinding", BindingName: "rb-role", RoleKind: "Role", RoleName: "some-role"}, refs["rb-role"])
}

func read[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return new(T), nil
		}
		return nil, err
	}
	object := new(T)
	if err = yaml.Unmarshal(data, object); err != nil {
		return nil, err
	}
	return object, nil
}

func render(templates map[string][]byte, project *v1alpha3.Project, projectTemplate *v1alpha1.ProjectTemplate, isFirstInstall bool) (*bytes.Buffer, error) {
	ch := buildChart(templates, project.Name)

	values := buildValues(project, projectTemplate, templateParameters(project, projectTemplate))
	valuesToRender, err := chartutil.ToRenderValues(ch, values, chartutil.ReleaseOptions{
		Name:      project.Name,
		Namespace: project.Name,
	}, nil)
	if err != nil {
		return nil, err
	}

	rendered, err := engine.Render(ch, valuesToRender)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	for _, file := range rendered {
		buf.WriteString(file)
	}

	return newPostRenderer(project, nil, ctrl.Log.WithName("test"), isFirstInstall).Run(buf)
}

// A render-only client renders the shipped chart and refuses to release or delete, instead of
// reaching for a cluster configuration it does not have.
func TestNewRenderOnly(t *testing.T) {
	t.Parallel()

	client, err := NewRenderOnly("../../helmlib", ctrl.Log.WithName("test"))
	require.NoError(t, err)

	project := &v1alpha3.Project{}
	project.Name = "test"
	template := caseTemplate(t, "simple")
	manifests, err := client.renderTemplate(project, template, templateParameters(project, template))
	require.NoError(t, err)
	assert.Contains(t, manifests, "kind: Namespace")

	_, err = client.UpgradeManifests(context.Background(), project, manifests, nil)
	require.ErrorIs(t, err, errRenderOnly)
	require.ErrorIs(t, client.Delete(context.Background(), project.Name), errRenderOnly)

	_, err = NewRenderOnly("./no-such-chart", ctrl.Log.WithName("test"))
	require.Error(t, err)
}

func TestReleaseName(t *testing.T) {
	t.Parallel()

	// Names within Helm's limit are used verbatim (no migration for existing releases).
	passthrough := []struct {
		name    string
		project string
	}{
		{name: "short name", project: "t-proj"},
		{name: "exactly the limit", project: strings.Repeat("a", helmReleaseNameMaxLen)},
	}
	for _, tt := range passthrough {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.project, ReleaseName(tt.project))
		})
	}

	// A name above the limit is shortened to a valid, deterministic, <=53-char release name.
	long := "t-" + strings.Repeat("z", 59) // 61 chars, above the 53 limit
	got := ReleaseName(long)
	assert.LessOrEqual(t, len(got), helmReleaseNameMaxLen, "release name must fit Helm's limit")
	assert.Equal(t, got, ReleaseName(long), "release name must be deterministic")
	assert.Regexp(t, `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`, got, "must be a valid Helm release name")

	// Distinct long names map to distinct release names (the suffix hashes the full name).
	other := "t-" + strings.Repeat("y", 59)
	assert.NotEqual(t, ReleaseName(long), ReleaseName(other))
}
