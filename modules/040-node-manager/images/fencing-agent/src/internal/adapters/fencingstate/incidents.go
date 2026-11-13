/*
Copyright 2026 Flant JSC

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

package fencingstate

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// Incidents creates, reads and deletes FencingFailedNodeState objects.
type Incidents struct {
	client client.Client
}

func NewIncidents(c client.Client) *Incidents {
	return &Incidents{client: c}
}

func (i *Incidents) CreateFFNS(ctx context.Context, ffns *v1alpha1.FencingFailedNodeState) error {
	if err := i.client.Create(ctx, ffns); err != nil {
		return fmt.Errorf("create fencingfailednodestate %q: %w", ffns.Name, err)
	}

	return nil
}

func (i *Incidents) GetFFNS(ctx context.Context, name string) (*v1alpha1.FencingFailedNodeState, error) {
	var ffns v1alpha1.FencingFailedNodeState
	if err := i.client.Get(ctx, types.NamespacedName{Name: name}, &ffns); err != nil {
		return nil, fmt.Errorf("get fencingfailednodestate %q: %w", name, err)
	}

	return &ffns, nil
}

// DeleteFFNS deletes the object only if its resourceVersion did not change.
func (i *Incidents) DeleteFFNS(ctx context.Context, name, resourceVersion string) error {
	ffns := &v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := i.client.Delete(ctx, ffns, client.Preconditions{ResourceVersion: &resourceVersion}); err != nil {
		return fmt.Errorf("delete fencingfailednodestate %q: %w", name, err)
	}

	return nil
}

// PatchStatus writes the status of ffns through the /status subresource. The
// merge patch has only the changes from old to ffns and the resourceVersion
// of old, so it fails with a conflict when the object changed after old. The
// parts of the status that the patch does not change stay as they are.
func (i *Incidents) PatchStatus(ctx context.Context, old, ffns *v1alpha1.FencingFailedNodeState) error {
	patch := client.MergeFromWithOptions(old, client.MergeFromWithOptimisticLock{})
	if err := i.client.Status().Patch(ctx, ffns, patch); err != nil {
		return fmt.Errorf("patch the status of fencingfailednodestate %q: %w", ffns.Name, err)
	}

	return nil
}

// Own watches the FencingFailedNodeState of the own node by name.
type Own struct {
	informer cache.SharedIndexInformer
	nodeName string
	// set gets each version of the object and nil when it is gone.
	set func(ffns *v1alpha1.FencingFailedNodeState)
}

func NewOwn(c client.WithWatch, nodeName string, set func(ffns *v1alpha1.FencingFailedNodeState)) (*Own, error) {
	selector := fields.OneTermEqualSelector("metadata.name", nodeName).String()

	lw := &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = selector
			list := &v1alpha1.FencingFailedNodeStateList{}

			return list, c.List(ctx, list, &client.ListOptions{Raw: &options})
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = selector

			return c.Watch(ctx, &v1alpha1.FencingFailedNodeStateList{}, &client.ListOptions{Raw: &options})
		},
	}

	o := &Own{
		informer: cache.NewSharedIndexInformer(cache.ToListWatcherWithWatchListSemantics(lw, c), &v1alpha1.FencingFailedNodeState{}, 0, cache.Indexers{}),
		nodeName: nodeName,
		set:      set,
	}

	if _, err := o.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { o.observe(obj, true) },
		UpdateFunc: func(_, obj any) { o.observe(obj, true) },
		DeleteFunc: func(obj any) { o.observe(obj, false) },
	}); err != nil {
		return nil, fmt.Errorf("add own fencingfailednodestate event handler: %w", err)
	}

	return o, nil
}

// Run watches until ctx ends. After the first sync it reports an absent object.
func (o *Own) Run(ctx context.Context) error {
	go o.informer.RunWithContext(ctx)

	if !cache.WaitForCacheSync(ctx.Done(), o.informer.HasSynced) {
		return nil
	}

	if len(o.informer.GetStore().ListKeys()) == 0 {
		o.set(nil)
	}

	<-ctx.Done()

	return nil
}

func (o *Own) observe(obj any, present bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	// The field selector is a server-side filter. Do not rely on it.
	ffns, ok := obj.(*v1alpha1.FencingFailedNodeState)
	if !ok || ffns.Name != o.nodeName {
		return
	}

	if !present {
		ffns = nil
	}

	o.set(ffns)
}
