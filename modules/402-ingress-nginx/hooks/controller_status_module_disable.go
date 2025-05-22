/*
Copyright 2025 Flant JSC

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
// f.ValuesSetFromYaml("global.enabledModules", []byte(`[ingress-nginx]`))

package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(handleStatusUpdaterForModuleDisable))

func handleStatusUpdaterForModuleDisable(input *go_hook.HookInput, dc dependency.Container) error {
	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")
	if !enabledModules.Has("ingress-nginx") {
		k8sClient, err := dc.GetK8sClient()
		if err != nil {
			return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
		}

		controllersList, err := k8sClient.Dynamic().Resource(schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1",
			Resource: "ingressnginxcontrollers",
		}).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error to read list of IngressNginxControllers: %w", err)
		}

		for _, unstructuredController := range controllersList.Items {
			controllerName, found, err := unstructured.NestedString(unstructuredController.Object, "metadata", "name")
			if err != nil || !found {
				return fmt.Errorf("failed to get metadata.name for controller: %v", err)
			}

			statusPatch := map[string]any{
				"status": map[string]any{
					"version": "unknown",
					"conditions": []map[string]any{
						{
							"type":           "Ready",
							"status":         "False",
							"lastUpdateTime": time.Now().Format(time.RFC3339),
							"reason":         "ModuleDisabled",
							"message":        "Ingress-nginx module is disabled",
						},
					},
				},
			}

			input.PatchCollector.PatchWithMerge(
				statusPatch,
				"deckhouse.io/v1",
				"IngressNginxController",
				"",
				controllerName,
				object_patch.WithSubresource("/status"),
			)

			keyValueForVersion := fmt.Sprintf("ingressNginx.internal.appliedControllerVersion.%s", controllerName)
			input.Values.Remove(keyValueForVersion)
		}
	}
	return nil
}
