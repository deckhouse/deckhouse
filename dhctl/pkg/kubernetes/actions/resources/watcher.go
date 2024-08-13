// Copyright 2024 Flant JSC
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
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Watcher struct {
	client   client.KubeClient
	isReady  func(object *unstructured.Unstructured) bool
	onDelete func(object *unstructured.Unstructured)
}

func NewWatcher(client client.KubeClient) *Watcher {
	return &Watcher{
		client: client,
	}
}

func (w *Watcher) WithIsReady(isReady func(object *unstructured.Unstructured) bool) *Watcher {
	w.isReady = isReady
	return w
}

func (w *Watcher) WithOnDelete(onDelete func(object *unstructured.Unstructured)) *Watcher {
	w.onDelete = onDelete
	return w
}

func (w *Watcher) Watch(ctx context.Context, resourceID ResourceID, onReady func(*unstructured.Unstructured, bool)) (err error) {
	if w.isReady == nil {
		w.isReady = isReady
	}

	if w.onDelete == nil {
		w.onDelete = func(_ *unstructured.Unstructured) {}
	}

	apiVersion, kind := resourceID.GroupVersionKind.ToAPIVersionAndKind()

	groupVersionResource, err := w.client.GroupVersionResource(apiVersion, kind)
	if err != nil {
		return fmt.Errorf("error getting GroupVersionResource: %w", err)
	}

	informer := dynamicinformer.NewFilteredDynamicInformer(
		w.client.Dynamic(),
		groupVersionResource,
		resourceID.Namespace,
		0,
		cache.Indexers{},
		func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", resourceID.Name).String()
		},
	)

	_, err = informer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(object interface{}) {
				unstructuredObject := object.(*unstructured.Unstructured)

				onReady(unstructuredObject, w.isReady(unstructuredObject))
			},
			UpdateFunc: func(_, object interface{}) {
				unstructuredObject := object.(*unstructured.Unstructured)

				onReady(unstructuredObject, w.isReady(unstructuredObject))
			},
			DeleteFunc: func(object interface{}) {
				unstructuredObject := object.(*unstructured.Unstructured)

				w.onDelete(unstructuredObject)
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	runCtx, runCancelFn := context.WithCancel(ctx)
	defer func() {
		panicValue := recover()
		if panicValue != nil {
			runCancelFn()

			panic(panicValue)
		}

		if err != nil {
			runCancelFn()
		}
	}()

	err = setWatchErrorHandler(runCancelFn, resourceID.String(), informer.Informer().SetWatchErrorHandler)
	if err != nil {
		return fmt.Errorf("failed to set watch error handler: %w", err)
	}

	go informer.Informer().Run(runCtx.Done())

	return nil
}

func isReady(object *unstructured.Unstructured) bool {
	status, ok := object.Object["status"].(map[string]interface{})
	if !ok {
		return true
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return true
	}

	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}

		if conditionMap["type"] == "Ready" && conditionMap["status"] == "False" {
			return false
		}
	}

	return true
}

func setWatchErrorHandler(cancelFn context.CancelFunc, resourceName string, setWatchErrorHandler func(handler cache.WatchErrorHandler) error) error {
	return setWatchErrorHandler(
		func(_ *cache.Reflector, err error) {
			switch {
			case apierrors.IsResourceExpired(err):
				log.InfoF("watch of %q closed with: %s\n", resourceName, err)
			case err == io.EOF:
				// watch closed normally
			case err == io.ErrUnexpectedEOF:
				log.InfoF("watch of %q closed with unexpected EOF: %s\n", resourceName, err)
			default:
				log.WarnF("failed to watch %q: %s\n", resourceName, err)
				cancelFn()
			}
		},
	)
}
