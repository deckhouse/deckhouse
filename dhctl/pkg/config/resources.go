package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type KubernetesResourceVersion struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type Resources struct {
	Items map[schema.GroupVersionKind]unstructured.UnstructuredList
}

func ParseResources(path string) (*Resources, error) {
	fileContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading resources file: %v", err)
	}

	bigFileTmp := strings.TrimSpace(string(fileContent))
	docs := regexp.MustCompile(`(?:^|\s*\n)---\s*`).Split(bigFileTmp, -1)

	resources := Resources{}
	resources.Items = make(map[schema.GroupVersionKind]unstructured.UnstructuredList)
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

		list, ok := resources.Items[gvk]
		if !ok {
			list = unstructured.UnstructuredList{}
		}

		list.Items = append(list.Items, kubernetesResource)
		resources.Items[gvk] = list
	}

	return &resources, nil
}
