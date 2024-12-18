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
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"

	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/validate"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"

	"sigs.k8s.io/yaml"
)

func Test(t *testing.T) {
	templates, err := parseHelmTemplates("../../templates")
	assert.Nil(t, err)
	for _, c := range []string{"empty_case", "secure_case", "secure_with_dedicated_node_case", "default_case", "without_ns_case"} {
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

	buf, err := render(templates, project, projectTemplate)
	if err != nil {
		return err
	}
	rendered := releaseutil.SplitManifests(buf.String())

	// uncomment for test and render rendered resources
	//os.WriteFile(filepath.Join(basePath, "rendered.yaml"), buf.Bytes(), 0644)

	rawExpected, err := os.ReadFile(filepath.Join(basePath, "resources.yaml"))
	if err != nil {
		return err
	}
	expected := releaseutil.SplitManifests(string(rawExpected))

	var renderedMap = make(map[string]interface{})
	for _, raw := range rendered {
		var object unstructured.Unstructured
		if err = yaml.Unmarshal([]byte(raw), &object); err != nil {
			return err
		}
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}
		renderedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = raw
	}

	var expectedMap = make(map[string]string)
	for _, raw := range expected {
		var object unstructured.Unstructured
		if err = yaml.Unmarshal([]byte(raw), &object); err != nil {
			return err
		}
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}
		expectedMap[fmt.Sprintf("%s.%s.%s.%s", object.GetAPIVersion(), object.GetKind(), object.GetNamespace(), object.GetName())] = raw
	}

	for name := range renderedMap {
		if _, ok := expectedMap[name]; ok {
			if diff := cmp.Diff(renderedMap[name], expectedMap[name]); diff != "" {
				fmt.Println(diff)
				return fmt.Errorf("rendered manifest '%s' doesn't match the expected manifest: %s", name, diff)
			}
		} else {
			return fmt.Errorf("rendered manifests don't match the expected manifests: resource '%s' not found", name)
		}
	}

	for name := range expectedMap {
		if _, ok := renderedMap[name]; ok {
			if diff := cmp.Diff(renderedMap[name], expectedMap[name]); diff != "" {
				fmt.Println(diff)
				return fmt.Errorf("expected '%s' manifest doesn't match the rendered manifests: %s", name, diff)
			}
		} else {
			return fmt.Errorf("expected manifests don't match the rendered manifests: resource '%s' not found", name)
		}
	}

	return nil
}

func read[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var object T
	if err = yaml.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	return &object, nil
}

func render(templates map[string][]byte, project *v1alpha2.Project, projectTemplate *v1alpha1.ProjectTemplate) (*bytes.Buffer, error) {
	ch, err := makeChart(templates, project.Name)
	if err != nil {
		return nil, err
	}

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
	return newPostRenderer(project.Name, projectTemplate.Name, ctrl.Log.WithName("test")).Run(buf)
}
