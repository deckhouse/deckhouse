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
// f.ValuesSetFromYaml("global.enabledModules", []byte(`[ingress-nginx]`))

package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(handleStatusUpdaterForModuleDisable))

var ingressNginxControllerGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "ingressnginxcontrollers",
}

func ingressNginxModuleEnabled(input *go_hook.HookInput) bool {
	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")
	return enabledModules.Has("ingress-nginx")
}

func listIngressNginxControllers(ctx context.Context, dc dependency.Container) (*unstructured.UnstructuredList, error) {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	controllersList, err := k8sClient.Dynamic().Resource(ingressNginxControllerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error to read list of IngressNginxControllers: %w", err)
	}

	return controllersList, nil
}

func getControllerName(unstructuredController *unstructured.Unstructured) (string, error) {
	controllerName, found, err := unstructured.NestedString(unstructuredController.Object, "metadata", "name")
	if err != nil || !found {
		return "", fmt.Errorf("failed to get metadata.name for controller: %v", err)
	}
	return controllerName, nil
}

func getControllerStatus(unstructuredController *unstructured.Unstructured, controllerName string) (IngressNginxControllerFilterResult, error) {
	// Reuse filter to keep the same parsing logic.
	// We intentionally don't use snapshots here: at module disable hook we list objects directly.
	result, err := filterIngressNginxController(unstructuredController)
	if err != nil {
		return IngressNginxControllerFilterResult{}, fmt.Errorf("failed to unmarshal IngressNginxController %q: %w", controllerName, err)
	}
	return result.(IngressNginxControllerFilterResult), nil
}

func moduleDisabledPatchNeeded(controllerStatus IngressNginxControllerFilterResult) bool {
	currentReadyCondition := findReadyCondition(controllerStatus.Conditions)
	return controllerStatus.Version != unknownVersion ||
		readyConditionNeedsUpdate(currentReadyCondition, conditionStatusFalse, conditionReasonModuleDisabled, conditionMessageModuleDisabled)
}

func buildModuleDisabledPatchStatus(now string) map[string]any {
	return map[string]any{
		"version": unknownVersion,
		"conditions": []map[string]any{
			{
				"type":           conditionTypeReady,
				"status":         conditionStatusFalse,
				"lastUpdateTime": now,
				"reason":         conditionReasonModuleDisabled,
				"message":        conditionMessageModuleDisabled,
			},
		},
	}
}

func clearAppliedControllerVersion(input *go_hook.HookInput, controllerName string) {
	keyValueForVersion := fmt.Sprintf(valuesAppliedControllerVersionFmt, controllerName)
	if input.Values.Exists(keyValueForVersion) {
		input.Values.Remove(keyValueForVersion)
	}
}

func handleStatusUpdaterForModuleDisable(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	if ingressNginxModuleEnabled(input) {
		return nil
	}

	controllersList, err := listIngressNginxControllers(ctx, dc)
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)

	for i := range controllersList.Items {
		unstructuredController := &controllersList.Items[i]

		controllerName, err := getControllerName(unstructuredController)
		if err != nil {
			return err
		}

		controllerStatus, err := getControllerStatus(unstructuredController, controllerName)
		if err != nil {
			return err
		}

		if moduleDisabledPatchNeeded(controllerStatus) {
			patchIngressNginxControllerStatus(input, controllerName, buildModuleDisabledPatchStatus(now))
		}

		clearAppliedControllerVersion(input, controllerName)
	}

	return nil
}
