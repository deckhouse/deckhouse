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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(migrateCPOCPNToNamespaced))

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

var cpoCPNClusterScopedCRDs = []string{
	"controlplaneoperations.control-plane.deckhouse.io",
	"controlplanenodes.control-plane.deckhouse.io",
}

func migrateCPOCPNToNamespaced(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	crdClient := client.Dynamic().Resource(crdGVR)

	for _, crdName := range cpoCPNClusterScopedCRDs {
		obj, err := crdClient.Get(context.Background(), crdName, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}

		scope, _, _ := unstructured.NestedString(obj.Object, "spec", "scope")
		if scope != "Cluster" {
			continue
		}

		input.Logger.Info("deleting cluster-scoped CRD to migrate to namespaced scope", "crd", crdName)
		if err := crdClient.Delete(context.Background(), crdName, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
