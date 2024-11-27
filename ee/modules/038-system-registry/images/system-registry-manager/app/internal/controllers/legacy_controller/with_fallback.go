/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *RegistryReconciler) listWithFallback(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	logger := ctrl.LoggerFrom(ctx)
	err := r.Client.List(ctx, list, opts...)
	// Error other than not found, return err
	if err != nil {
		return err
	}

	// Can't extract list items, return err
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}

	// Object found in cache, return
	if len(items) > 0 {
		return nil
	}

	logger.Info("Object not found in cache, trying to List directly")
	return r.APIReader.List(ctx, list, opts...)
}
