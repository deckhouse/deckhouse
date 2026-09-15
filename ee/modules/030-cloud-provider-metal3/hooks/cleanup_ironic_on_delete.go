/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const metal3ProviderNamespace = "d8-cloud-provider-metal3"

var ironicGVR = schema.GroupVersionResource{
	Group:    "ironic.metal3.io",
	Version:  "v1alpha1",
	Resource: "ironics",
}

var removeFinalizersPatch = map[string]interface{}{
	"metadata": map[string]interface{}{
		"finalizers": nil,
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(cleanupIronicOnDelete))

func cleanupIronicOnDelete(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	ironics, err := k8sClient.Dynamic().Resource(ironicGVR).Namespace(metal3ProviderNamespace).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	patch, err := json.Marshal(removeFinalizersPatch)
	if err != nil {
		return err
	}

	for _, ironic := range ironics.Items {
		name := ironic.GetName()

		_, err = k8sClient.Dynamic().Resource(ironicGVR).Namespace(metal3ProviderNamespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		input.Logger.Info("Ironic finalizers removed",
			slog.String("namespace", metal3ProviderNamespace),
			slog.String("name", name),
		)

		if ironic.GetDeletionTimestamp() != nil {
			continue
		}

		err = k8sClient.Dynamic().Resource(ironicGVR).Namespace(metal3ProviderNamespace).Delete(ctx, name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		input.Logger.Info("Ironic deleted",
			slog.String("namespace", metal3ProviderNamespace),
			slog.String("name", name),
		)
	}

	return nil
}
