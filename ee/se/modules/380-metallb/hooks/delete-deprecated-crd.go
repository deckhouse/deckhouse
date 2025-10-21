/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(removeDeprecatedCRD))

func removeDeprecatedCRD(_ context.Context, _ *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	_, err = kubeClient.Dynamic().Resource(gvr).Get(context.TODO(), "addresspools.metallb.io", metav1.GetOptions{})
	if err != nil {
		return nil
	}

	err = kubeClient.Dynamic().Resource(gvr).Delete(context.TODO(), "addresspools.metallb.io", metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("cannot delete CRD: %v", err)
	}
	return nil
}
