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

// syncInstanceClassConsumers publishes status.nodeGroupConsumers on every registered
// InstanceClass: the sorted names of the CloudEphemeral NodeGroups pointing at it. A class
// nobody references gets an empty list, which is what the deletion webhook reads as "free".
func syncInstanceClassConsumers(ctx context.Context, c client.Client) error {
	gvks, err := nodecommon.RegisteredInstanceClassGVKs(ctx, c)
	if err != nil {
		return fmt.Errorf("list registered instance class kinds: %w", err)
	}
	if len(gvks) == 0 {
		return nil
	}

	ngList := &v1.NodeGroupList{}
	if err := c.List(ctx, ngList); err != nil {
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
	// Sorted so the published order does not depend on the list order, which would otherwise
	// churn the patch and the printer column on every pass.
	for key := range consumers {
		slices.Sort(consumers[key])
	}

	var errs []error
	for _, gvk := range gvks {
		if err := applyConsumersForKind(ctx, c, gvk, consumers); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func applyConsumersForKind(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, consumers map[usageKey][]string) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := c.List(ctx, list); err != nil {
		// A provider can register before its CRD is served; the kind joins on a later pass.
		if meta.IsNoMatchError(err) {
			log.FromContext(ctx).V(1).Info("instance class kind is not served yet", "gvk", gvk.String())
			return nil
		}
		return fmt.Errorf("list %s at %s: %w", gvk.Kind, gvk.Version, err)
	}

	for i := range list.Items {
		item := &list.Items[i]
		desired := consumers[usageKey{Kind: gvk.Kind, Name: item.GetName()}]
		if desired == nil {
			desired = make([]string, 0)
		}

		// A malformed field falls through to the patch that repairs it.
		current, found, err := unstructured.NestedStringSlice(item.Object, "status", "nodeGroupConsumers")
		if err == nil && found && slices.Equal(current, desired) {
			continue
		}

		body, err := json.Marshal(map[string]any{
			"status": map[string]any{"nodeGroupConsumers": desired},
		})
		if err != nil {
			return fmt.Errorf("marshal consumers patch for %s/%s: %w", gvk.Kind, item.GetName(), err)
		}
		// RawPatch rather than MergeFrom: a merge patch built from the object would carry
		// `annotations: null` and wipe the annotations of the class.
		if err := c.Patch(ctx, item, client.RawPatch(types.MergePatchType, body)); err != nil {
			return fmt.Errorf("patch %s/%s consumers: %w", gvk.Kind, item.GetName(), err)
		}
	}
	return nil
}
