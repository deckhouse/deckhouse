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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
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

func Test(t *testing.T) {
	templates, err := parseHelmTemplates("../../helmlib")
	assert.Nil(t, err)
	for _, c := range []string{"default_case", "secure_case", "secure_with_dedicated_node_case", "simple_case", "empty_case", "without_ns_case", "skip_heritage_and_unmanaged_case"} {
		t.Run(c, func(t *testing.T) {
			basePath := filepath.Join("./testdata", c)
			assert.Nil(t, test(templates, basePath))
		})
	}
}

func test(templates map[string][]byte, basePath string) error {
	projectTemplate, err := read[v1alpha1.ProjectTemplate](filepath.Join(basePath, "template.yaml"))
	if err != nil {
		return err
	}

	if err = validate.ProjectTemplate(projectTemplate); err != nil {
		return err
	}

	project, err := read[v1alpha3.Project](filepath.Join(basePath, "project.yaml"))
	if err != nil {
		return err
	}

	if err = validate.Project(project, projectTemplate); err != nil {
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

func TestLegacyResourcesFiltered(t *testing.T) {
	templates, err := parseHelmTemplates("../../helmlib")
	assert.Nil(t, err)

	basePath := filepath.Join("./testdata", "legacy_filtered_case")
	projectTemplate, err := read[v1alpha1.ProjectTemplate](filepath.Join(basePath, "template.yaml"))
	assert.Nil(t, err)

	project, err := read[v1alpha3.Project](filepath.Join(basePath, "project.yaml"))
	assert.Nil(t, err)

	ch := buildChart(templates, project.Name)
	valuesToRender, err := chartutil.ToRenderValues(ch, buildValues(project, projectTemplate), chartutil.ReleaseOptions{
		Name:      project.Name,
		Namespace: project.Name,
	}, nil)
	assert.Nil(t, err)

	rendered, err := engine.Render(ch, valuesToRender)
	assert.Nil(t, err)

	buf := bytes.NewBuffer(nil)
	for _, file := range rendered {
		buf.WriteString(file)
	}

	post := newPostRenderer(project, nil, ctrl.Log.WithName("test"), true)
	out, err := post.Run(buf)
	assert.Nil(t, err)

	// the post-renderer must report that controller-managed kinds were dropped
	assert.True(t, post.filtered, "expected ResourceQuota/AuthorizationRule to be filtered out")

	renderedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range releaseutil.SplitManifests(out.String()) {
		object, ok, parseErr := parseManifest(raw)
		assert.Nil(t, parseErr)
		if !ok {
			continue
		}
		renderedMap[fmt.Sprintf("%s.%s", object.GetKind(), object.GetName())] = object
	}

	// filtered kinds must be gone
	_, hasRQ := renderedMap["ResourceQuota.all-pods"]
	assert.False(t, hasRQ, "ResourceQuota must be filtered out")
	_, hasAR := renderedMap["AuthorizationRule.legacy-admin"]
	assert.False(t, hasAR, "AuthorizationRule must be filtered out")

	// non-filtered resources must remain
	_, hasNS := renderedMap["Namespace.test"]
	assert.True(t, hasNS, "project namespace must be present")
	_, hasCM := renderedMap["ConfigMap.keep-me"]
	assert.True(t, hasCM, "regular resources must be kept")
}

// TestListWrapperCannotBypassFilter guards H5: a filtered kind (ResourceQuota) smuggled inside a
// kind: List must still be dropped, while a sibling ConfigMap inside the same List is kept.
func TestListWrapperCannotBypassFilter(t *testing.T) {
	manifest := `
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: sneaky
    spec:
      hard:
        pods: "10"
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: keep-me
    data:
      a: b
`
	project := &v1alpha3.Project{}
	project.Name = "test"

	post := newPostRenderer(project, nil, ctrl.Log.WithName("test"), true)
	out, err := post.Run(bytes.NewBufferString(manifest))
	assert.Nil(t, err)
	assert.True(t, post.filtered, "ResourceQuota inside a List must be filtered out")

	renderedMap := make(map[string]*unstructured.Unstructured)
	for _, raw := range releaseutil.SplitManifests(out.String()) {
		object, ok, parseErr := parseManifest(raw)
		assert.Nil(t, parseErr)
		if !ok {
			continue
		}
		renderedMap[fmt.Sprintf("%s.%s", object.GetKind(), object.GetName())] = object
	}

	_, hasRQ := renderedMap["ResourceQuota.sneaky"]
	assert.False(t, hasRQ, "ResourceQuota smuggled in a List must be filtered out")
	_, hasCM := renderedMap["ConfigMap.keep-me"]
	assert.True(t, hasCM, "ConfigMap inside the List must be kept")
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

// TestMergeWithDefaults_AdditionalProperties pins the surprising-but-intentional behaviour of the
// additionalProperties branch: a schema with additionalProperties models a free-form map, so the
// user's keys are passed through verbatim and any named-property defaults are discarded.
func TestMergeWithDefaults_AdditionalProperties(t *testing.T) {
	schema := &spec.Schema{}
	schema.Properties = map[string]spec.Schema{
		"declared": {SchemaProps: spec.SchemaProps{Default: "from-schema"}},
	}
	schema.AdditionalProperties = &spec.SchemaOrBool{Allows: true}

	out := mergeWithDefaults(schema, map[string]any{"free": "value", "declared": "kept"})

	// only the non-property key survives; the declared property and its schema default are dropped.
	assert.Equal(t, map[string]any{"free": "value"}, out)
}

// TestMergeWithDefaults_ObjectDefaultPreserved pins the behaviour the schema-based ProjectTemplate
// render relies on: an object-typed property whose default is the whole object is kept verbatim (the
// merge only recurses into sub-properties when there is no default). Structured templates carry their
// fixed values (namespaceMetadata, dedicatedNodes, allowedUIDs) as such object defaults.
func TestMergeWithDefaults_ObjectDefaultPreserved(t *testing.T) {
	schema := &spec.Schema{}
	schema.Properties = map[string]spec.Schema{
		"namespace": {SchemaProps: spec.SchemaProps{
			Type:    spec.StringOrArray{"object"},
			Default: map[string]any{"labels": map[string]any{"team": "x"}},
			Properties: map[string]spec.Schema{
				"labels": {SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"object"}}},
			},
		}},
	}

	out := mergeWithDefaults(schema, map[string]any{})
	assert.Equal(t, map[string]any{"labels": map[string]any{"team": "x"}}, out["namespace"])
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

	valuesToRender, err := chartutil.ToRenderValues(ch, buildValues(project, projectTemplate), chartutil.ReleaseOptions{
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
