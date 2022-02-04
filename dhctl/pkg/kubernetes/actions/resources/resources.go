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

package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func CreateResources(kubeCl *client.KubernetesClient, resources *template.Resources) error {
	for gvk := range resources.Items {
		var resourcesList *metav1.APIResourceList
		var err error
		err = retry.NewSilentLoop("Get resources list", 25, 5*time.Second).Run(func() error {
			resourcesList, err = kubeCl.Discovery().ServerResourcesForGroupVersion(gvk.GroupVersion().String())
			if err != nil {
				return fmt.Errorf("can't get preferred resources: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, discoveredResource := range resourcesList.APIResources {
			if discoveredResource.Kind != gvk.Kind {
				continue
			}
			if err := createSingleResource(kubeCl, resources, gvk); err != nil {
				return err
			}
			delete(resources.Items, gvk)
			break
		}
	}

	resourcesToCreate := make([]string, 0, len(resources.Items))
	for key := range resources.Items {
		resourcesToCreate = append(resourcesToCreate, key.String())
	}

	if len(resourcesToCreate) > 0 {
		log.InfoF("\rResources to create: \n\t%s\n\n", strings.Join(resourcesToCreate, "\n\t"))
	}
	return nil
}

func isNamespaced(kubeCl *client.KubernetesClient, gvk schema.GroupVersionKind, name string) (bool, error) {
	lists, err := kubeCl.APIResourceList(gvk.GroupVersion().String())
	if err != nil && len(lists) == 0 {
		// apiVersion is defined and there is a ServerResourcesForGroupVersion error
		return false, err
	}

	namespaced := false
	for _, list := range lists {
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			if resource.Name == name {
				namespaced = resource.Namespaced
				break
			}
		}
	}
	return namespaced, nil
}

func createSingleResource(kubeCl *client.KubernetesClient, resources *template.Resources, gvk schema.GroupVersionKind) error {
	return retry.NewLoop(fmt.Sprintf("Create %s resources", gvk.String()), 25, 5*time.Second).Run(func() error {
		gvr, err := kubeCl.GroupVersionResource(gvk.ToAPIVersionAndKind())
		if err != nil {
			return fmt.Errorf("can't get resource by kind and apiVersion: %w", err)
		}

		namespaced, err := isNamespaced(kubeCl, gvk, gvr.Resource)
		if err != nil {
			return fmt.Errorf("can't determine whether a resource is namespaced or not: %v", err)
		}

		item := resources.Items[gvk]
		for _, doc := range item.Items {
			docCopy := doc.DeepCopy()
			namespace := docCopy.GetNamespace()
			if namespace == metav1.NamespaceNone && namespaced {
				namespace = metav1.NamespaceDefault
			}

			manifestTask := actions.ManifestTask{
				Name:     getUnstructuredName(docCopy),
				Manifest: func() interface{} { return nil },
				CreateFunc: func(manifest interface{}) error {
					_, err := kubeCl.Dynamic().Resource(gvr).
						Namespace(namespace).
						Create(context.TODO(), docCopy, metav1.CreateOptions{})
					return err
				},
				UpdateFunc: func(manifest interface{}) error {
					content, err := docCopy.MarshalJSON()
					if err != nil {
						return err
					}
					// using patch here because of https://github.com/kubernetes/kubernetes/issues/70674
					_, err = kubeCl.Dynamic().Resource(gvr).
						Namespace(namespace).
						Patch(context.TODO(), docCopy.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
					return err
				},
			}
			if err := manifestTask.CreateOrUpdate(); err != nil {
				return err
			}
		}
		return nil
	})
}

func CreateResourcesLoop(kubeCl *client.KubernetesClient, resources *template.Resources) error {
	timeout, err := time.ParseDuration(app.ResourcesTimeout)
	if err != nil {
		return fmt.Errorf("cannot parse timeout to create resources: %v", err)
	}
	endChannel := time.After(timeout)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		err := CreateResources(kubeCl, resources)
		if err != nil {
			return err
		}

		if len(resources.Items) == 0 {
			return nil
		}

		select {
		case <-endChannel:
			return fmt.Errorf("creating resources failed after %s waiting", app.ResourcesTimeout)
		case <-ticker.C:
		}
	}
}

func getUnstructuredName(obj *unstructured.Unstructured) string {
	namespace := obj.GetNamespace()
	if namespace == "" {
		return fmt.Sprintf("%s %s", obj.GetKind(), obj.GetName())
	}
	return fmt.Sprintf("%s %s/%s", obj.GetKind(), obj.GetNamespace(), obj.GetName())
}
