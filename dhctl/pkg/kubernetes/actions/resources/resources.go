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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var ErrNotAllResourcesCreated = fmt.Errorf("Not all resources were created")

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

func (g *apiResourceListGetter) Get(ctx context.Context, gvk *schema.GroupVersionKind) (*metav1.APIResourceList, error) {
	key := gvk.GroupVersion().String()
	if resourcesList, ok := g.gvkToResourcesList[key]; ok {
		return resourcesList, nil
	}

	var resourcesList *metav1.APIResourceList
	var err error
	err = retry.NewSilentLoop("Get resources list", 3, 1*time.Second).RunContext(ctx, func() error {
		// ServerResourcesForGroupVersion does not return error if API returned NotFound (404) or Forbidden (403)
		// https://github.com/kubernetes/client-go/blob/51a4fd4aee686931f6a53148b3f4c9094f80d512/discovery/discovery_client.go#L204
		// and if CRD was not deployed method will return empty APIResources list
		resourcesList, err = g.kubeCl.Discovery().ServerResourcesForGroupVersion(gvk.GroupVersion().String())
		if err != nil {
			return fmt.Errorf("can't get preferred resources '%s': %w", key, err)
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
	mcTasks   []actions.ModuleConfigTask
}

func NewCreator(kubeCl *client.KubernetesClient, resources template.Resources, tasks []actions.ModuleConfigTask) *Creator {
	return &Creator{
		kubeCl:    kubeCl,
		resources: resources,
		mcTasks:   tasks,
	}
}

func (c *Creator) createAll(ctx context.Context) error {
	apiResourceGetter := newAPIResourceListGetter(c.kubeCl)
	addedResourcesIndexes := make(map[int]struct{})

	defer func() {
		remainResources := make([]*template.Resource, 0)

		for i, resource := range c.resources {
			if _, ok := addedResourcesIndexes[i]; !ok {
				log.DebugF("Remain resource %s\n", resource.String())
				remainResources = append(remainResources, resource)
			}
		}

		c.resources = remainResources
		log.DebugF("Remain resources: %d\n", len(c.resources))
	}()

	log.DebugLn("start ensureRequiredNamespacesExist")

	// resourcesToSkipInCurrentIteration connect with c.resources via resource slice index
	resourcesToSkipInCurrentIteration, err := c.ensureRequiredNamespacesExist(ctx)
	if err != nil {
		return err
	}

	log.DebugLn("start single resource creation loop")

	for indx, resource := range c.resources {
		if _, shouldSkip := resourcesToSkipInCurrentIteration[indx]; shouldSkip {
			log.DebugF("Resource %s with index % should skip to create in current iteration because namespace is not existed\n", resource.String())
			continue
		}

		resourcesList, err := apiResourceGetter.Get(ctx, &resource.GVK)
		if err != nil {
			log.DebugF("apiResourceGetter returns error: %w\n", err)
			continue
		}

		for _, discoveredResource := range resourcesList.APIResources {
			if discoveredResource.Kind != resource.GVK.Kind {
				continue
			}
			if err := c.createSingleResource(ctx, resource, discoveredResource); err != nil {
				return err
			}

			addedResourcesIndexes[indx] = struct{}{}
			break
		}
	}

	return nil
}

func (c *Creator) ensureRequiredNamespacesExist(ctx context.Context) (map[int]struct{}, error) {
	// true means known existing namespace
	// false means known namespace that is not yet created (used to skip checking for that namespace for multiple times)
	knownNamespaces := make(map[string]bool)
	// we need to skip all resources without existing namespace
	// because namespace can possibly be created in current iteration
	// or after state is set to "cluster is bootstrapped" (some namespaces will be created by the deckhouse after that)
	resourcesToSkipInCurrentIteration := make(map[int]struct{})

	err := retry.NewSilentLoop("Ensure that required namespaces exist", 10, 10*time.Second).RunContext(ctx, func() error {
		for i, res := range c.resources {
			nsName := res.Object.GetNamespace()

			if nsName == "" {
				// we can receive empty name space when user want to deploy in 'default' ns
				// we keep it in our minds and skip verify therese resources because we think that
				// default namespace always exist
				log.DebugF("Namespace is empty for resource %s. Skip ns checking\n", res.String())
				continue
			}

			namespaceExists, nsWasSeenBefore := knownNamespaces[nsName]
			if nsWasSeenBefore {
				if !namespaceExists {
					// we have two cases; first - we check namespace and namespace is existed and not
					// if ns is existed then we will skip only
					// if ns is not exists we should skip resource on current iteration and try to create on next iteration
					resourcesToSkipInCurrentIteration[i] = struct{}{}
					log.DebugF("Namespace not found but processed for resource %s. Adding skip to create resource in current iteration\n", res.String())
				}
				log.DebugF("Namespace was processed for resource %s. Skip ns checking\n", res.String())
				continue
			}

			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if _, err := c.kubeCl.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{}); err != nil {
				cancel()

				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("can't get namespace %q: %w", nsName, err)
				}

				resourcesToSkipInCurrentIteration[i] = struct{}{}
				knownNamespaces[nsName] = false
				log.DebugF("Namespace was not found for resource %s\n", res.String())
				continue
			}
			cancel()
			knownNamespaces[nsName] = true
			log.DebugF("Namespace found for resource %s\n", res.String())
		}
		return nil
	})

	if err != nil {
		return make(map[int]struct{}), err
	}

	return resourcesToSkipInCurrentIteration, nil
}

func (c *Creator) TryToCreate(ctx context.Context) error {
	if err := c.createAll(ctx); err != nil {
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

	for _, task := range c.mcTasks {
		err := c.runSingleMCTask(ctx, task)
		if err != nil {
			return err
		}
	}

	// we do not want to support same creation logic for module config tasks as for resources
	// if task was failed we return error.
	// thus, all tasks were done here, just remove tasks for prevent multiple applying
	if len(c.mcTasks) > 0 {
		c.mcTasks = make([]actions.ModuleConfigTask, 0)
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

func resourceToGVR(resource *template.Resource, apires metav1.APIResource) (*schema.GroupVersionResource, *unstructured.Unstructured, error) {
	doc := resource.Object

	gvr := &schema.GroupVersionResource{
		Group:    resource.GVK.Group,
		Version:  resource.GVK.Version,
		Resource: apires.Name,
	}

	namespaced := isNamespacedByAPIRes(apires)

	docCopy := doc.DeepCopy()
	namespace := docCopy.GetNamespace()
	if namespace == metav1.NamespaceNone && namespaced {
		namespace = metav1.NamespaceDefault
	}

	docCopy.SetNamespace(namespace)

	return gvr, docCopy, nil
}

func (c *Creator) createSingleResource(ctx context.Context, resource *template.Resource, apires metav1.APIResource) error {
	// Wait up to 10 minutes
	return retry.NewLoop(fmt.Sprintf("Create %s resources", resource.GVK.String()), 60, 10*time.Second).RunContext(ctx, func() error {
		gvr, docCopy, err := resourceToGVR(resource, apires)
		if err != nil {
			return err
		}
		namespace := docCopy.GetNamespace()
		manifestTask := actions.ManifestTask{
			Name:     getUnstructuredName(docCopy),
			Manifest: func() interface{} { return nil },
			CreateFunc: func(manifest interface{}) error {
				_, err := c.kubeCl.Dynamic().Resource(*gvr).
					Namespace(namespace).
					Create(ctx, docCopy, metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				content, err := docCopy.MarshalJSON()
				if err != nil {
					return err
				}
				// using patch here because of https://github.com/kubernetes/kubernetes/issues/70674
				_, err = c.kubeCl.Dynamic().Resource(*gvr).
					Namespace(namespace).
					Patch(ctx, docCopy.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
				return err
			},
		}

		err = manifestTask.CreateOrUpdate()
		if err != nil {
			if strings.Contains(err.Error(), "the server could not find the requested resource") {
				c.kubeCl.InvalidateDiscoveryCache()
			}
		}
		return err
	})
}

func (c *Creator) runSingleMCTask(ctx context.Context, task actions.ModuleConfigTask) error {
	// Wait up to 10 minutes
	return retry.NewLoop(task.Title, 60, 5*time.Second).RunContext(ctx, func() error {
		return task.Do(c.kubeCl)
	})
}

func CreateResourcesLoop(ctx context.Context, kubeCl *client.KubernetesClient, resources template.Resources, checkers []Checker, tasks []actions.ModuleConfigTask) error {
	endChannel := time.After(app.ResourcesTimeout)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	resourceCreator := NewCreator(kubeCl, resources, tasks)

	waiter := NewWaiter(checkers)
	for {
		err := resourceCreator.TryToCreate(ctx)
		if err != nil && !errors.Is(err, ErrNotAllResourcesCreated) {
			return err
		}

		ready, errWaiter := waiter.ReadyAll(ctx)
		if errWaiter != nil {
			return errWaiter
		}

		if ready && err == nil {
			return nil
		}

		select {
		case <-endChannel:
			if len(resources) > 0 {
				return fmt.Errorf(
					"Creating resources timed out after %s: resources cannot become ready. "+
						"This could be due to lack of worker nodes in the cluster. "+
						"Add at least one worker node or remove taints from master nodes (for single-node cluster) ",
					app.ResourcesTimeout,
				)
			}

			return fmt.Errorf("Creating resources failed after %s waiting", app.ResourcesTimeout)
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

		var resourceClient dynamic.ResourceInterface
		if namespaced {
			resourceClient = kubeCl.Dynamic().Resource(gvr).Namespace(namespace)
		} else {
			resourceClient = kubeCl.Dynamic().Resource(gvr)
		}

		if err := resourceClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("unable to delete %s %s: %w", gvr.String(), name, err)
			}
			log.DebugF("Unable to delete resource: %s %s: %s\n", gvr.String(), name, err)
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

	for _, list := range lists {
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			if resource.Name == name {
				return resource.Namespaced, nil
			}
		}
	}
	return false, nil
}

func isNamespacedByAPIRes(res metav1.APIResource) bool {
	if len(res.Verbs) == 0 {
		return false
	}
	return res.Namespaced
}
