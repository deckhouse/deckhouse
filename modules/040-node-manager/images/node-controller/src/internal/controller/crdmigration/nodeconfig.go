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

package crdmigration

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// MigrateNodeConfigs drops the string spec.extensions[].requestedBy a node-controller from before
// the contract freeze wrote: the typed cache cannot decode it, and the next render writes the
// object form. One-off for clusters on a main build of that time, no release carries the string.
func MigrateNodeConfigs(ctx context.Context, c client.Client) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(internalv1alpha1.GroupVersion.WithKind("NodeConfigList"))
	if err := c.List(ctx, list); err != nil {
		return fmt.Errorf("list NodeConfigs: %w", err)
	}

	for i := range list.Items {
		config := &list.Items[i]
		extensions, _, err := unstructured.NestedSlice(config.Object, "spec", "extensions")
		if err != nil {
			return fmt.Errorf("read the extensions of NodeConfig %s: %w", config.GetName(), err)
		}

		legacy := false
		for _, item := range extensions {
			extension, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if _, isString := extension["requestedBy"].(string); isString {
				delete(extension, "requestedBy")
				legacy = true
			}
		}
		if !legacy {
			continue
		}

		// A merge patch, not an apply: an apply types the live object first, and the string fails it.
		patch, err := json.Marshal(map[string]any{
			"metadata": map[string]any{"resourceVersion": config.GetResourceVersion()},
			"spec":     map[string]any{"extensions": extensions},
		})
		if err != nil {
			return fmt.Errorf("build the patch of NodeConfig %s: %w", config.GetName(), err)
		}
		if err := c.Patch(ctx, config, client.RawPatch(types.MergePatchType, patch)); err != nil {
			return fmt.Errorf("migrate NodeConfig %s: %w", config.GetName(), err)
		}
	}

	return nil
}
