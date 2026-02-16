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
	"controller/apis/deckhouse.io/v1alpha2"
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
	for _, c := range []string{"default_case", "secure_case", "secure_with_dedicated_node_case", "empty_case", "without_ns_case", "skip_heritage_and_unmanaged_case"} {
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

	project, err := read[v1alpha2.Project](filepath.Join(basePath, "project.yaml"))
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
	//os.WriteFile(filepath.Join(basePath, "resources.yaml"), buf.Bytes(), 0644)

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

	project, err := read[v1alpha2.Project](filepath.Join(basePath, "project.yaml"))
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
	if labels[v1alpha2.ResourceLabelHeritage] != "" {
		t.Errorf("unmanaged resource should not have heritage label, but got: %s", labels[v1alpha2.ResourceLabelHeritage])
	}
	if labels[v1alpha2.ResourceLabelProject] != project.Name {
		t.Errorf("unmanaged resource should have project label, got: %s", labels[v1alpha2.ResourceLabelProject])
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

	project, err := read[v1alpha2.Project](filepath.Join(basePath, "project.yaml"))
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

func render(templates map[string][]byte, project *v1alpha2.Project, projectTemplate *v1alpha1.ProjectTemplate, isFirstInstall bool) (*bytes.Buffer, error) {
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
