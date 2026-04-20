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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// TODO: Remove this migration hook after v1alpha1 is dropped from IngressNginxController CRD.

const (
	ingressNginxControllerAPIVersionMigrationAnnotation = "ingress-nginx.deckhouse.io/migrated-api-version"
	ingressNginxControllerAPIVersionTarget              = "deckhouse.io/v1"
)

type ingressNginxControllerAPIVersionMigration struct {
	Name               string
	Namespace          string
	MigratedAPIVersion string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress_nginx_controllers",
			ApiVersion: ingressNginxControllerAPIVersionTarget,
			Kind:       "IngressNginxController",
			FilterFunc: applyIngressNginxControllerAPIVersionMigrationFilter,
		},
	},
}, migrateIngressNginxControllerAPIVersion)

func applyIngressNginxControllerAPIVersionMigrationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return ingressNginxControllerAPIVersionMigration{
		Name:               obj.GetName(),
		Namespace:          obj.GetNamespace(),
		MigratedAPIVersion: obj.GetAnnotations()[ingressNginxControllerAPIVersionMigrationAnnotation],
	}, nil
}

func migrateIngressNginxControllerAPIVersion(_ context.Context, input *go_hook.HookInput) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				ingressNginxControllerAPIVersionMigrationAnnotation: ingressNginxControllerAPIVersionTarget,
			},
		},
	}

	for controller, err := range sdkobjectpatch.SnapshotIter[ingressNginxControllerAPIVersionMigration](input.Snapshots.Get("ingress_nginx_controllers")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ingress_nginx_controllers' snapshot: %w", err)
		}

		if controller.MigratedAPIVersion == ingressNginxControllerAPIVersionTarget {
			continue
		}

		input.PatchCollector.PatchWithMerge(
			patch,
			ingressNginxControllerAPIVersionTarget,
			"IngressNginxController",
			controller.Namespace,
			controller.Name,
			object_patch.WithIgnoreMissingObject(),
		)
	}

	return nil
}
