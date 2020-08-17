package deckhouse

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/flant/logboek"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/util/retry"
)

func CreateResources(kubeCl *kube.KubernetesClient, resources *config.Resources) error {
	for gvk := range resources.Items {
		var resourcesList *metav1.APIResourceList
		var err error
		err = retry.StartSilentLoop("Get resources list", 25, 5, func() error {
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
		logboek.LogInfoF("\rResources to create: \n\t%s\n\n", strings.Join(resourcesToCreate, "\n\t"))
	}
	return nil
}

func createSingleResource(kubeCl *kube.KubernetesClient, resources *config.Resources, gvk schema.GroupVersionKind) error {
	return retry.StartLoop(fmt.Sprintf("Creation of %s resources", gvk.String()), 25, 5, func() error {
		gvr, err := kubeCl.GroupVersionResource(gvk.ToAPIVersionAndKind())
		if err != nil {
			return fmt.Errorf("can't get resource by kind and apiVersion: %w", err)
		}

		item := resources.Items[gvk]
		for _, doc := range item.Items {
			docCopy := doc.DeepCopy()
			name := getUnstructuredName(docCopy)
			err := runTask(createManifestTask{
				name:     name,
				manifest: func() interface{} { return nil },
				createTask: func(manifest interface{}) error {
					_, err := kubeCl.Dynamic().Resource(gvr).Create(docCopy, metav1.CreateOptions{})
					return err
				},
				updateTask: func(manifest interface{}) error {
					content, err := docCopy.MarshalJSON()
					if err != nil {
						return err
					}
					// using patch here because of https://github.com/kubernetes/kubernetes/issues/70674
					_, err = kubeCl.Dynamic().Resource(gvr).Patch(docCopy.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
					return err
				},
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func CreateResourcesLoop(kubeCl *kube.KubernetesClient, resources *config.Resources) error {
	endChannel := time.After(15 * time.Minute)
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
			return fmt.Errorf("creating resources failed after 15m waiting")
		case <-time.After(10 * time.Second):
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
