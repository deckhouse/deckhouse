// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type KubernetesResourceVersion struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type Resource struct {
	GVK    schema.GroupVersionKind
	Object unstructured.Unstructured
}

type Resources struct {
	Items []*Resource
}

func ParseResources(path string, data map[string]interface{}) (*Resources, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading resources file: %v", err)
	}

	content := string(fileContent)

	if data != nil {
		t := template.New("resource_render").Funcs(FuncMap())
		t, err := t.Parse(content)
		if err != nil {
			return nil, err
		}

		var tpl bytes.Buffer

		err = t.Execute(&tpl, data)
		if err != nil {
			return nil, err
		}

		content = tpl.String()
	}

	bigFileTmp := strings.TrimSpace(content)
	docs := regexp.MustCompile(`(?:^|\s*\n)---\s*`).Split(bigFileTmp, -1)

	resources := Resources{}
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var kubernetesResource unstructured.Unstructured
		err := yaml.Unmarshal([]byte(doc), &kubernetesResource)
		if err != nil {
			return nil, fmt.Errorf("parsing doc \n%s\n: %v", doc, err)
		}

		gvk := schema.FromAPIVersionAndKind(kubernetesResource.GetAPIVersion(), kubernetesResource.GetKind())

		resources.Items = append(resources.Items, &Resource{
			GVK:    gvk,
			Object: kubernetesResource,
		})
	}

	return &resources, nil
}
