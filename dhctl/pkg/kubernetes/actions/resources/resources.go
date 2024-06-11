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
	"errors"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

var ErrNotAllResourcesCreated = fmt.Errorf("Not all resources were creatated")

// apiResourceListGetter discovery and cache APIResources list for group version kind
type apiResourceListGetter struct {
	kubeCl             *client.KubernetesClient
	gvkToResourcesList map[string]*metav1.APIResourceList
}

func newAPIResourceListGetter(kubeCl *client.KubernetesClient) *apiResourceListGetter {
	return &apiResourceListGetter{
		kubeCl:             kubeCl,
		gvkToResourcesList: make(map[string]*metav1.APIResourceList),
	}
}

func (g *apiResourceListGetter) Get(gvk *schema.GroupVersionKind) (*metav1.APIResourceList, error) {
	key := gvk.GroupVersion().String()
	if resourcesList, ok := g.gvkToResourcesList[key]; ok {
		return resourcesList, nil
	}

	var resourcesList *metav1.APIResourceList
	var err error
	err = retry.NewSilentLoop("Get resources list", 50, 5*time.Second).Run(func() error {
		// ServerResourcesForGroupVersion does not return error if API returned NotFound (404) or Forbidden (403)
		// https://github.com/kubernetes/client-go/blob/51a4fd4aee686931f6a53148b3f4c9094f80d512/discovery/discovery_client.go#L204
		// and if CRD was not deployed method will return empty APIResources list
		resourcesList, err = g.kubeCl.Discovery().ServerResourcesForGroupVersion(gvk.GroupVersion().String())
		if err != nil {
			return fmt.Errorf("can't get preferred resources: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return resourcesList, nil
}

type Creator struct {
	kubeCl    *client.KubernetesClient
	resources []*template.Resource
}

func NewCreator(kubeCl *client.KubernetesClient, resources template.Resources) *Creator {
	return &Creator{
		kubeCl:    kubeCl,
		resources: resources,
	}
}

func (c *Creator) createAll() error {
	apiResourceGetter := newAPIResourceListGetter(c.kubeCl)
	addedResourcesIndexes := make(map[int]struct{})

	defer func() {
		remainResources := make([]*template.Resource, 0)

		for i, resource := range c.resources {
			if _, ok := addedResourcesIndexes[i]; !ok {
				remainResources = append(remainResources, resource)
			}
		}

		c.resources = remainResources
	}()

	for indx, resource := range c.resources {
		resourcesList, err := apiResourceGetter.Get(&resource.GVK)
		if err != nil {
			return err
		}

		for _, discoveredResource := range resourcesList.APIResources {
			if discoveredResource.Kind != resource.GVK.Kind {
				continue
			}
			if err := c.createSingleResource(resource); err != nil {
				return err
			}

			addedResourcesIndexes[indx] = struct{}{}
			break
		}
	}

	return nil
}

func (c *Creator) TryToCreate() error {
	if err := c.createAll(); err != nil {
		return err
	}

	gvks := make(map[string]struct{})
	resourcesToCreate := make([]string, 0, len(c.resources))
	for _, resource := range c.resources {
		key := resource.GVK.String()
		if _, ok := gvks[key]; !ok {
			gvks[key] = struct{}{}
			resourcesToCreate = append(resourcesToCreate, key)
		}
	}

	if len(c.resources) > 0 {
		log.InfoF("\rResources to create: \n\t%s\n\n", strings.Join(resourcesToCreate, "\n\t"))
		return ErrNotAllResourcesCreated
	}

	return nil
}

func (c *Creator) isNamespaced(gvk schema.GroupVersionKind, name string) (bool, error) {
	return isNamespaced(c.kubeCl, gvk, name)
}

func (c *Creator) createSingleResource(resource *template.Resource) error {
	doc := resource.Object
	gvk := resource.GVK

	// Wait up to 10 minutes
	return retry.NewLoop(fmt.Sprintf("Create %s resources", gvk.String()), 60, 10*time.Second).Run(func() error {
		gvr, err := c.kubeCl.GroupVersionResource(gvk.ToAPIVersionAndKind())
		if err != nil {
			return fmt.Errorf("can't get resource by kind and apiVersion: %w", err)
		}

		namespaced, err := c.isNamespaced(gvk, gvr.Resource)
		if err != nil {
			return fmt.Errorf("can't determine whether a resource is namespaced or not: %v", err)
		}

		docCopy := doc.DeepCopy()
		namespace := docCopy.GetNamespace()
		if namespace == metav1.NamespaceNone && namespaced {
			namespace = metav1.NamespaceDefault
		}

		manifestTask := actions.ManifestTask{
			Name:     getUnstructuredName(docCopy),
			Manifest: func() interface{} { return nil },
			CreateFunc: func(manifest interface{}) error {
				_, err := c.kubeCl.Dynamic().Resource(gvr).
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
				_, err = c.kubeCl.Dynamic().Resource(gvr).
					Namespace(namespace).
					Patch(context.TODO(), docCopy.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
				return err
			},
		}

		return manifestTask.CreateOrUpdate()
	})
}

func CreateResourcesLoop(kubeCl *client.KubernetesClient, resources template.Resources, checkers []Checker) error {
	endChannel := time.After(app.ResourcesTimeout)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	resourceCreator := NewCreator(kubeCl, resources)

	waiter := NewWaiter(checkers)

	for {
		err := resourceCreator.TryToCreate()
		if err != nil && !errors.Is(err, ErrNotAllResourcesCreated) {
			return err
		}

		ready, errWaiter := waiter.ReadyAll()
		if errWaiter != nil {
			return errWaiter
		}

		if ready && err == nil {
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

func DeleteResourcesLoop(ctx context.Context, kubeCl *client.KubernetesClient, resources template.Resources) error {
	for _, res := range resources {
		name := res.Object.GetName()
		namespace := res.Object.GetNamespace()
		gvk := res.GVK

		gvr, err := kubeCl.GroupVersionResource(gvk.ToAPIVersionAndKind())
		if err != nil {
			return fmt.Errorf("bad group version resource %s: %w", res.GVK.String(), err)
		}

		namespaced, err := isNamespaced(kubeCl, gvk, gvr.Resource)
		if err != nil {
			return fmt.Errorf("can't determine whether a resource is namespaced or not: %v", err)
		}
		if namespace == metav1.NamespaceNone && namespaced {
			namespace = metav1.NamespaceDefault
		}

		if namespaced {
			log.InfoF("Deleting %s %s in ns/%s\n", gvr, name, namespace)
			if err := kubeCl.Dynamic().Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("unable to remove %s %s: %w", gvr.String(), name, err)
				}
			}
		} else {
			log.InfoF("Deleting %s %s\n", gvr, name)
			if err := kubeCl.Dynamic().Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("unable to remove %s %s: %w", gvr.String(), name, err)
				}
			}
		}
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
