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
	"os"
	"sort"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

var kindOrderMap map[string]int

var bootstrapKindsOrder = []string{
	"Namespace",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
	"ServiceAccount",
	"Secret",
	"ConfigMap",
	"StorageClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"CustomResourceDefinition",
	"ClusterRole",
	"ClusterRoleBinding",
	"Role",
	"RoleBinding",
	"Service",
	"DaemonSet",
	"Pod",
	"ReplicationController",
	"ReplicaSet",
	"Deployment",
	"StatefulSet",
	"Job",
	"CronJob",
	"Ingress",
	"APIService",
}

func init() {
	kindOrderMap = make(map[string]int)

	for i, k := range bootstrapKindsOrder {
		kindOrderMap[k] = i + 1
	}
}

type KubernetesResourceVersion struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type Resource struct {
	GVK    schema.GroupVersionKind
	Object unstructured.Unstructured
}

type Resources []*Resource

func (r Resources) Len() int {
	return len(r)
}

func (r Resources) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r Resources) Less(i, j int) bool {
	firstResource := r[i]
	secondResource := r[j]
	firstOrder, firstOk := kindOrderMap[firstResource.GVK.Kind]
	secondOrder, secondOk := kindOrderMap[secondResource.GVK.Kind]
	// if same kind (including unknown) sub sort alphanumeric
	if firstOrder == secondOrder {
		// if both are unknown save order
		if !firstOk && !secondOk {
			return false
		}
		// otherwise, sort by name
		return firstResource.Object.GetName() < secondResource.Object.GetName()
	}

	// unknown kind is last
	if !firstOk {
		return false
	}
	if !secondOk {
		return true
	}
	// sort different kinds
	return firstOrder < secondOrder
}

func ParseResourcesContent(content string, data map[string]interface{}) (Resources, error) {
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

	docs := BigFileSplit(content)

	// false-positive for `Consider preallocating `resources` (prealloc)`
	//nolint:prealloc
	var resources Resources = make([]*Resource, 0, len(docs))
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

		if gvk.Empty() || gvk.GroupVersion().Empty() || gvk.GroupKind().Empty() {
			log.WarnF("Empty gvr for resource:\n%s\n", doc)
		}

		resources = append(resources, &Resource{
			GVK:    gvk,
			Object: kubernetesResource,
		})
	}

	sort.Stable(resources)

	return resources, nil
}

func loadResources(path string, data map[string]interface{}) (Resources, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading resources file: %v", err)
	}

	content := string(fileContent)

	return ParseResourcesContent(content, data)
}

func ParseResources(path string, data map[string]interface{}) (Resources, error) {
	resources, err := loadResources(path, data)
	if err != nil {
		return nil, err
	}

	return resources, nil
}

func BigFileSplit(content string) []string {
	bigFileTmp := strings.TrimSpace(content)
	return input.YAMLSplitRegexp.Split(bigFileTmp, -1)
}
