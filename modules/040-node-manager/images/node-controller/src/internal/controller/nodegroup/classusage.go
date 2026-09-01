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

package nodegroup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// usageKey identifies an InstanceClass the way a NodeGroup's classReference names it.
type usageKey struct {
	Kind string
	Name string
}

// sweepInstanceClassConsumers coalesces the calls of the reconcile workers: one sweep runs at a
// time, a caller that finds one running marks it dirty and returns, and the running sweep repeats
// once more if any mark arrived while it was working.
func (r *Status) sweepInstanceClassConsumers(ctx context.Context) {
	r.sweepMu.Lock()
	if r.sweeping {
		r.sweepDirty = true
		r.sweepMu.Unlock()
		return
	}
	r.sweeping = true
	r.sweepMu.Unlock()

	// From a defer, so a panic in the sweep leaves neither the flag set for the life of the pod
	// nor the mark this pass consumed unswept: an unwound pass swept nothing.
	swept := false
	defer func() {
		r.sweepMu.Lock()
		r.sweeping = false
		if !swept {
			r.sweepDirty = true
		}
		r.sweepMu.Unlock()
	}()

	for {
		if err := r.syncInstanceClassConsumers(ctx); err != nil {
			log.FromContext(ctx).Error(err, "sync instance class consumers")
		}

		r.sweepMu.Lock()
		if !r.sweepDirty {
			swept = true
			r.sweepMu.Unlock()
			return
		}
		r.sweepDirty = false
		r.sweepMu.Unlock()
	}
}

// syncInstanceClassConsumers publishes status.nodeGroupConsumers on every registered
// InstanceClass: the sorted names of the CloudEphemeral NodeGroups pointing at it. A class that
// lost its consumers is cleared to an empty list; a class that never had the field keeps none.
func (r *Status) syncInstanceClassConsumers(ctx context.Context) error {
	gvks, err := nodecommon.RegisteredInstanceClassGVKs(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("list registered instance class kinds: %w", err)
	}
	if len(gvks) == 0 {
		return nil
	}

	ngList := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return fmt.Errorf("list nodegroups: %w", err)
	}

	consumers := map[usageKey][]string{}
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral || ng.Spec.CloudInstances == nil {
			continue
		}
		ref := ng.Spec.CloudInstances.ClassReference
		if ref.Kind == "" || ref.Name == "" {
			continue
		}
		key := usageKey{Kind: ref.Kind, Name: ref.Name}
		consumers[key] = append(consumers[key], ng.Name)
	}
	// Sorted, so the published order does not depend on the order the NodeGroups were listed in.
	for key := range consumers {
		slices.Sort(consumers[key])
	}

	var errs []error
	for _, gvk := range gvks {
		if err := r.applyConsumersForKind(ctx, gvk, consumers); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *Status) applyConsumersForKind(ctx context.Context, gvk schema.GroupVersionKind, consumers map[usageKey][]string) error {
	if r.unstorableConsumers[gvk] {
		return nil
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := r.Client.List(ctx, list); err != nil {
		// A provider can register before its CRD is served: skip the kind, the next sweep
		// retries it.
		if meta.IsNoMatchError(err) {
			log.FromContext(ctx).V(1).Info("instance class kind is not served yet", "gvk", gvk.String())
			return nil
		}
		return fmt.Errorf("list %s at %s: %w", gvk.Kind, gvk.Version, err)
	}

	var errs []error
	for i := range list.Items {
		item := &list.Items[i]
		desired := consumers[usageKey{Kind: gvk.Kind, Name: item.GetName()}]
		if desired == nil {
			desired = make([]string, 0)
		}

		// An absent field compares equal to an empty list, so a class nobody references is
		// left alone. A malformed one falls through to the patch that repairs it.
		current, _, err := unstructured.NestedStringSlice(item.Object, "status", "nodeGroupConsumers")
		if err == nil && slices.Equal(current, desired) {
			continue
		}

		body, err := json.Marshal(map[string]any{
			"status": map[string]any{"nodeGroupConsumers": desired},
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("marshal consumers patch for %s/%s: %w", gvk.Kind, item.GetName(), err))
			continue
		}
		// The body is built here rather than diffed from the object, so the patch carries this
		// one field and nothing else.
		if err := r.Client.Patch(ctx, item, client.RawPatch(types.MergePatchType, body)); err != nil {
			errs = append(errs, fmt.Errorf("patch %s/%s consumers: %w", gvk.Kind, item.GetName(), err))
			continue
		}

		// An accepted write that does not come back in the response is a kind that will not
		// keep the field, so it is dropped after this one attempt. The DVP CRD schema declares
		// no status at all: modules/030-cloud-provider-dvp/crds/instance_class.yaml.
		stored, _, err := unstructured.NestedStringSlice(item.Object, "status", "nodeGroupConsumers")
		if err != nil || !slices.Equal(stored, desired) {
			r.markConsumersUnstorable(ctx, gvk)
			break
		}
	}
	return errors.Join(errs...)
}

func (r *Status) markConsumersUnstorable(ctx context.Context, gvk schema.GroupVersionKind) {
	if r.unstorableConsumers == nil {
		r.unstorableConsumers = map[schema.GroupVersionKind]bool{}
	}
	r.unstorableConsumers[gvk] = true
	log.FromContext(ctx).Info("instance class kind does not keep status.nodeGroupConsumers, skipping it from now on: "+
		"its classes publish no consumers and the deletion webhook stops blocking their deletion "+
		"(internal/webhook/instanceclass_webhook.go reads len(status.nodeGroupConsumers)); "+
		"the CRD schema has to declare the field, as in modules/030-cloud-provider-dvp/crds/instance_class.yaml it does not",
		"gvk", gvk.String())
}
