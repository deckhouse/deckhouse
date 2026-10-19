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

package nodeconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// machineFields are the spec paths the machine owns (withoutMachineFields): never
// applied by node-controller and never taken over from the writer that set them.
var machineFields = fieldpath.NewSet(
	fieldpath.MakePathOrDie("spec", "statusToken"),
	fieldpath.MakePathOrDie("spec", "network"),
	fieldpath.MakePathOrDie("spec", "storage"),
	fieldpath.MakePathOrDie("spec", "kubelet", "registerWithTaints"),
)

// takeOver hands fieldManager the spec fields other writers own by update (the node registering
// its object, a cloud node's create, a hand edit), the machine's fields aside: a field two
// managers own stays when one stops applying it, and a value the render dropped would stay.
func (r *Reconciler) takeOver(ctx context.Context, existing *internalv1alpha1.NodeConfig, logger logr.Logger) (*internalv1alpha1.NodeConfig, error) {
	entries, taken, err := takeOverSpec(existing.ManagedFields, metav1.NewTime(time.Now()))
	if err != nil {
		return nil, fmt.Errorf("read the field owners of NodeConfig %s: %w", existing.Name, err)
	}
	if !taken {
		return existing, nil
	}
	// The revision goes with the owners, as in client-go's csaupgrade: owners computed
	// from one revision must not overwrite another's.
	patch, err := json.Marshal([]map[string]any{
		{"op": "replace", "path": "/metadata/managedFields", "value": entries},
		{"op": "replace", "path": "/metadata/resourceVersion", "value": existing.ResourceVersion},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal the field owners of NodeConfig %s: %w", existing.Name, err)
	}
	updated := existing.DeepCopy()
	if err := r.Client.Patch(ctx, updated, client.RawPatch(types.JSONPatchType, patch)); err != nil {
		if apierrors.IsConflict(err) {
			logger.V(1).Info("NodeConfig changed while its fields were being taken over; leaving it to the next pass", "node", existing.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("take over the spec of NodeConfig %s: %w", existing.Name, err)
	}
	logger.Info("NodeConfig spec fields taken over from the writers that set them", "node", existing.Name)
	return updated, nil
}

// takeOverSpec moves the non-machine spec fields of every update entry of the main
// resource into fieldManager's apply entry, creating it if needed; an entry left with
// nothing is dropped. Reports false when no entry owned such a field.
func takeOverSpec(entries []metav1.ManagedFieldsEntry, now metav1.Time) ([]metav1.ManagedFieldsEntry, bool, error) {
	spec := fieldpath.MakePrefixMatcherOrDie("spec")
	taken := &fieldpath.Set{}
	kept := make([]metav1.ManagedFieldsEntry, 0, len(entries))
	own := -1
	for _, entry := range entries {
		if entry.Subresource != "" || entry.Operation != metav1.ManagedFieldsOperationUpdate {
			if entry.Subresource == "" && entry.Manager == fieldManager {
				own = len(kept)
			}
			kept = append(kept, entry)
			continue
		}
		fields, err := decodeFields(entry)
		if err != nil {
			return nil, false, fmt.Errorf("decode the fields of %s: %w", entry.Manager, err)
		}
		specFields := fields.FilterIncludeMatches(spec).RecursiveDifference(machineFields)
		if specFields.Empty() {
			kept = append(kept, entry)
			continue
		}
		taken = taken.Union(specFields)
		rest := fields.Difference(specFields)
		if rest.Empty() {
			continue
		}
		if err := encodeFields(&entry, rest); err != nil {
			return nil, false, fmt.Errorf("encode the fields of %s: %w", entry.Manager, err)
		}
		kept = append(kept, entry)
	}
	if taken.Empty() {
		return entries, false, nil
	}

	if own < 0 {
		kept = append(kept, metav1.ManagedFieldsEntry{
			Manager:    fieldManager,
			Operation:  metav1.ManagedFieldsOperationApply,
			APIVersion: internalv1alpha1.GroupVersion.String(),
			Time:       &now,
			FieldsType: "FieldsV1",
		})
		own = len(kept) - 1
	}
	owned, err := decodeFields(kept[own])
	if err != nil {
		return nil, false, fmt.Errorf("decode the fields of %s: %w", fieldManager, err)
	}
	if err := encodeFields(&kept[own], owned.Union(taken)); err != nil {
		return nil, false, fmt.Errorf("encode the fields of %s: %w", fieldManager, err)
	}
	return kept, true, nil
}

func decodeFields(entry metav1.ManagedFieldsEntry) (*fieldpath.Set, error) {
	fields := &fieldpath.Set{}
	if entry.FieldsV1 == nil {
		return fields, nil
	}
	if err := fields.FromJSON(bytes.NewReader(entry.FieldsV1.Raw)); err != nil {
		return nil, fmt.Errorf("parse fieldsV1: %w", err)
	}
	return fields, nil
}

func encodeFields(entry *metav1.ManagedFieldsEntry, fields *fieldpath.Set) error {
	raw, err := fields.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize fieldsV1: %w", err)
	}
	entry.FieldsV1 = &metav1.FieldsV1{Raw: raw}
	return nil
}
