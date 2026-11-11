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

// TODO: remove in 1.78, every cluster has been migrated by then.

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	// userAuthnAPIListenersMigratedAnnotation on the kube-system Namespace records that the migration has run.
	userAuthnAPIListenersMigratedAnnotation = "control-plane-manager.deckhouse.io/user-authn-api-listeners-migrated"
	userAuthnNamespace                      = "d8-user-authn"
	userAuthnListenerSetName                = "user-authn"
)

var listenerSetGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "listenersets",
}

// userAuthnAPIListeners are the listeners user-authn published the API through before 1.77.
var userAuthnAPIListeners = []string{"api", "api-redirect"}

// In 1.77 the API is published by this module instead of user-authn, but the ListenerSet of user-authn
// keeps its api listeners until the user-authn release is upgraded, and this module is upgraded first.
// The Gateway rejects the listeners of this module as overlapping with them, and the upgrade never gets
// to user-authn, so remove them here once. user-authn no longer renders them.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(migrateUserAuthnAPIListeners))

func migrateUserAuthnAPIListeners(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %w", err)
	}

	// Without the Namespace user-authn has never been deployed, so there is nothing to migrate.
	_, err = kubeClient.CoreV1().Namespaces().Get(ctx, userAuthnNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot get Namespace %s: %w", userAuthnNamespace, err)
	}

	kubeSystem, err := kubeClient.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot get the kube-system Namespace: %w", err)
	}
	if _, ok := kubeSystem.Annotations[userAuthnAPIListenersMigratedAnnotation]; ok {
		input.Logger.Debug("user-authn API listeners have already been migrated")
		return nil
	}

	if err := removeUserAuthnAPIListeners(ctx, input, kubeClient.Dynamic()); err != nil {
		return err
	}

	markerPatch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{userAuthnAPIListenersMigratedAnnotation: "true"},
		},
	})
	if err != nil {
		return fmt.Errorf("cannot marshal the kube-system Namespace patch: %w", err)
	}
	_, err = kubeClient.CoreV1().Namespaces().Patch(ctx, "kube-system", types.MergePatchType, markerPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("cannot annotate the kube-system Namespace: %w", err)
	}

	return nil
}

func removeUserAuthnAPIListeners(ctx context.Context, input *go_hook.HookInput, dynamicClient dynamic.Interface) error {
	listenerSets := dynamicClient.Resource(listenerSetGVR).Namespace(userAuthnNamespace)

	// Without the Gateway API or without the ListenerSet there is nothing to overlap with.
	listenerSet, err := listenerSets.Get(ctx, userAuthnListenerSetName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot get ListenerSet %s/%s: %w", userAuthnNamespace, userAuthnListenerSetName, err)
	}

	listeners, _, err := unstructured.NestedSlice(listenerSet.Object, "spec", "listeners")
	if err != nil {
		return fmt.Errorf("cannot read the listeners of ListenerSet %s/%s: %w", userAuthnNamespace, userAuthnListenerSetName, err)
	}

	// Removed from the end, so that the indexes of the listeners still to be removed stay valid. Each
	// index is tested against the name it was read with, so the patch fails instead of removing another
	// listener if the list has changed since.
	var patch []map[string]any
	var removed []string
	for i := len(listeners) - 1; i >= 0; i-- {
		listener, _ := listeners[i].(map[string]any)
		name, _ := listener["name"].(string)
		if !slices.Contains(userAuthnAPIListeners, name) {
			continue
		}
		path := fmt.Sprintf("/spec/listeners/%d", i)
		patch = append(patch,
			map[string]any{"op": "test", "path": path + "/name", "value": name},
			map[string]any{"op": "remove", "path": path},
		)
		removed = append(removed, name)
	}
	if len(patch) == 0 {
		return nil
	}

	patchData, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("cannot marshal ListenerSet %s/%s patch: %w", userAuthnNamespace, userAuthnListenerSetName, err)
	}

	_, err = listenerSets.Patch(ctx, userAuthnListenerSetName, types.JSONPatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("cannot remove the API listeners from ListenerSet %s/%s: %w", userAuthnNamespace, userAuthnListenerSetName, err)
	}

	input.Logger.Info("Removed the API listeners from the user-authn ListenerSet",
		slog.String("namespace", userAuthnNamespace),
		slog.String("name", userAuthnListenerSetName),
		slog.Any("listeners", removed))

	return nil
}
