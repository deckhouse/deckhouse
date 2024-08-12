/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helm

import (
	"bytes"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
	"testing"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/validate"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"

	"sigs.k8s.io/yaml"
)

func Test(t *testing.T) {
	templates, err := parseHelmTemplates("../../templates")
	assert.Nil(t, err)
	for _, c := range []string{"secure_case", "default_case", "without_ns_case"} {
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

	for name, _ := range renderedMap {
		if _, ok := expectedMap[name]; ok {
			if diff := cmp.Diff(renderedMap[name], expectedMap[name]); diff != "" {
				fmt.Println(diff)
				return fmt.Errorf("rendered manifests don't match the expected manifests: %s", diff)
			}
		} else {
			return fmt.Errorf("rendered manifests don't match the expected manifests: resource '%s' not found", name)
		}
	}

	for name, _ := range expectedMap {
		if _, ok := renderedMap[name]; ok {
			if diff := cmp.Diff(renderedMap[name], expectedMap[name]); diff != "" {
				fmt.Println(diff)
				return fmt.Errorf("rendered manifests don't match the expected manifests: %s", diff)
			}
		} else {
			return fmt.Errorf("rendered manifests match the expected manifests: resource '%s' not found", name)
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
	projectName := project.Name
	templateName := projectTemplate.Name
	values := valuesFromProjectAndTemplate(project, projectTemplate)

	ch, err := makeChart(templates, projectName)
	if err != nil {
		return nil, err
	}

	valuesToRender, err := chartutil.ToRenderValues(ch, values, chartutil.ReleaseOptions{
		Name:      projectName,
		Namespace: projectName,
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
	return newPostRenderer(projectName, templateName).Run(buf)
}
