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
	"strings"
	"time"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ingressNginxNamespace = "d8-ingress-nginx"

	daemonSetSnapshotKey  = "ingress-nginx-daemonset"
	controllerSnapshotKey = "ingress-nginx-controller"

	ingressNginxControllerAPIVersion = "deckhouse.io/v1"
	ingressNginxControllerKind       = "IngressNginxController"

	controllerVersionAnnotation = "ingress-nginx-controller.deckhouse.io/controller-version"

	controllerNamePrefix    = "controller-"
	controllerNameLabelKey  = "name"
	controllerAppLabelValue = "controller"

	unknownVersion = "unknown"

	valuesAppliedControllerVersionRoot = "ingressNginx.internal.appliedControllerVersion"
	valuesAppliedControllerVersionFmt  = valuesAppliedControllerVersionRoot + ".%s"

	conditionTypeReady             = "Ready"
	conditionStatusTrue            = "True"
	conditionStatusFalse           = "False"
	conditionReasonAllPodsReady    = "AllPodsReady"
	conditionReasonPodsNotReady    = "PodsNotReady"
	conditionReasonModuleDisabled  = "ModuleDisabled"
	conditionMessageAllPodsReady   = "All controller pods are ready"
	conditionMessagePodsNotReady   = "Controller pods are not ready"
	conditionMessageModuleDisabled = "Ingress-nginx module is disabled"
)

type DaemonSet struct {
	Metadata struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
		Labels      map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		NumberReady            int64 `json:"numberReady"`
		DesiredNumberScheduled int64 `json:"desiredNumberScheduled"`
		UpdatedNumberScheduled int64 `json:"updatedNumberScheduled"`
	} `json:"status"`
}

type DaemonSetFilterResult struct {
	ControllerVersion string
	Name              string
	ControllerName    string
	NumberReady       int64
	DesiredNumber     int64
	UpdatedNumber     int64
}

type IngressNginxControllerCondition struct {
	Type           string `json:"type"`
	Status         string `json:"status"`
	LastUpdateTime string `json:"lastUpdateTime"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
}

type desiredControllerStatus struct {
	Version            string
	ObservedGeneration int64
	Ready              bool
}

type IngressNginxController struct {
	Metadata struct {
		Generation int64  `json:"generation"`
		Name       string `json:"name"`
	} `json:"metadata"`
	Status struct {
		ObservedGeneration int64                             `json:"observedGeneration"`
		Version            string                            `json:"version"`
		Conditions         []IngressNginxControllerCondition `json:"conditions"`
	} `json:"status"`
}

type IngressNginxControllerFilterResult struct {
	Name               string
	Generation         int64
	ObservedGeneration int64
	Version            string
	Conditions         []IngressNginxControllerCondition
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       daemonSetSnapshotKey,
			ApiVersion: "apps.kruise.io/v1alpha1",
			Kind:       "DaemonSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{ingressNginxNamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": controllerAppLabelValue,
				},
			},
			FilterFunc: filterIngressNginxDaemonset,
		},

		{
			Name:       controllerSnapshotKey,
			ApiVersion: ingressNginxControllerAPIVersion,
			Kind:       ingressNginxControllerKind,
			FilterFunc: filterIngressNginxController,
		},
	},
}, handleStatusUpdater)

func filterIngressNginxDaemonset(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ds DaemonSet
	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, err
	}

	controllerVersion := unknownVersion
	if version, exists := ds.Metadata.Annotations[controllerVersionAnnotation]; exists {
		controllerVersion = version
	}

	controllerName := strings.TrimPrefix(ds.Metadata.Name, controllerNamePrefix)
	if name, ok := ds.Metadata.Labels[controllerNameLabelKey]; ok && name != "" {
		controllerName = name
	}

	return DaemonSetFilterResult{
		ControllerVersion: controllerVersion,
		Name:              ds.Metadata.Name,
		ControllerName:    controllerName,
		NumberReady:       ds.Status.NumberReady,
		DesiredNumber:     ds.Status.DesiredNumberScheduled,
		UpdatedNumber:     ds.Status.UpdatedNumberScheduled,
	}, nil
}

func filterIngressNginxController(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var controller IngressNginxController
	err := sdk.FromUnstructured(obj, &controller)
	if err != nil {
		return nil, err
	}

	return IngressNginxControllerFilterResult{
		Name:               controller.Metadata.Name,
		Generation:         controller.Metadata.Generation,
		ObservedGeneration: controller.Status.ObservedGeneration,
		Version:            controller.Status.Version,
		Conditions:         controller.Status.Conditions,
	}, nil
}

func findReadyCondition(conditions []IngressNginxControllerCondition) *IngressNginxControllerCondition {
	for i := range conditions {
		if conditions[i].Type == conditionTypeReady {
			return &conditions[i]
		}
	}
	return nil
}

func readyConditionNeedsUpdate(current *IngressNginxControllerCondition, desiredStatus, desiredReason, desiredMessage string) bool {
	if current == nil {
		return true
	}
	return current.Status != desiredStatus || current.Reason != desiredReason || current.Message != desiredMessage
}

func ensureAppliedControllerVersionRoot(input *go_hook.HookInput) {
	if !input.Values.Exists(valuesAppliedControllerVersionRoot) {
		input.Values.Set(valuesAppliedControllerVersionRoot, map[string]any{})
	}
}

func indexControllersByName(input *go_hook.HookInput) (map[string]IngressNginxControllerFilterResult, error) {
	controllerSnapshots := input.Snapshots.Get(controllerSnapshotKey)

	controllersByName := make(map[string]IngressNginxControllerFilterResult, len(controllerSnapshots))
	for controllerInfo, err := range sdkobjectpatch.SnapshotIter[IngressNginxControllerFilterResult](controllerSnapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over %q snapshots: %w", controllerSnapshotKey, err)
		}
		controllersByName[controllerInfo.Name] = controllerInfo
	}

	return controllersByName, nil
}

func getAppliedControllerVersion(input *go_hook.HookInput, controllerName string) (string, bool) {
	keyValueForVersion := fmt.Sprintf(valuesAppliedControllerVersionFmt, controllerName)
	val, ok := input.Values.GetOk(keyValueForVersion)
	if !ok {
		return "", false
	}
	return val.String(), true
}

func isDaemonSetReadyAndUpdated(ds DaemonSetFilterResult) bool {
	if ds.DesiredNumber <= 0 {
		return false
	}
	return ds.NumberReady == ds.DesiredNumber && ds.UpdatedNumber == ds.DesiredNumber
}

func calculateDesiredControllerStatus(
	ds DaemonSetFilterResult,
	controller IngressNginxControllerFilterResult,
	appliedVersion string,
	hasAppliedVersion bool,
) (desired desiredControllerStatus, shouldSetAppliedVersion bool) {
	desired = desiredControllerStatus{
		Version:            unknownVersion,
		ObservedGeneration: controller.ObservedGeneration,
		Ready:              false,
	}

	if hasAppliedVersion {
		desired.Version = appliedVersion
	}

	if !isDaemonSetReadyAndUpdated(ds) {
		return desired, false
	}

	desired.Version = ds.ControllerVersion
	desired.ObservedGeneration = controller.Generation
	desired.Ready = true

	return desired, !hasAppliedVersion || appliedVersion != desired.Version
}

func buildControllerStatusPatch(
	current IngressNginxControllerFilterResult,
	desired desiredControllerStatus,
	now string,
) map[string]any {
	patchStatus := make(map[string]any)

	if current.Version != desired.Version {
		patchStatus["version"] = desired.Version
	}
	if current.ObservedGeneration != desired.ObservedGeneration {
		patchStatus["observedGeneration"] = desired.ObservedGeneration
	}

	desiredConditionStatus := conditionStatusFalse
	desiredReason := conditionReasonPodsNotReady
	desiredMessage := conditionMessagePodsNotReady
	if desired.Ready {
		desiredConditionStatus = conditionStatusTrue
		desiredReason = conditionReasonAllPodsReady
		desiredMessage = conditionMessageAllPodsReady
	}

	currentReadyCondition := findReadyCondition(current.Conditions)
	if readyConditionNeedsUpdate(currentReadyCondition, desiredConditionStatus, desiredReason, desiredMessage) {
		patchStatus["conditions"] = []map[string]any{
			{
				"type":           conditionTypeReady,
				"status":         desiredConditionStatus,
				"lastUpdateTime": now,
				"reason":         desiredReason,
				"message":        desiredMessage,
			},
		}
	}

	if len(patchStatus) == 0 {
		return nil
	}

	return patchStatus
}

func patchIngressNginxControllerStatus(input *go_hook.HookInput, controllerName string, patchStatus map[string]any) {
	input.PatchCollector.PatchWithMerge(
		map[string]any{"status": patchStatus},
		ingressNginxControllerAPIVersion,
		ingressNginxControllerKind,
		"",
		controllerName,
		object_patch.WithSubresource("/status"),
	)
}

func setAppliedControllerVersion(input *go_hook.HookInput, controllerName, version string) {
	keyValueForVersion := fmt.Sprintf(valuesAppliedControllerVersionFmt, controllerName)
	input.Values.Set(keyValueForVersion, version)
}

func handleStatusUpdater(_ context.Context, input *go_hook.HookInput) error {
	ensureAppliedControllerVersionRoot(input)

	now := time.Now().Format(time.RFC3339)

	controllersByName, err := indexControllersByName(input)
	if err != nil {
		return err
	}

	daemonSetSnapshots := input.Snapshots.Get(daemonSetSnapshotKey)
	for daemonSetInfo, err := range sdkobjectpatch.SnapshotIter[DaemonSetFilterResult](daemonSetSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over %q snapshots: %w", daemonSetSnapshotKey, err)
		}

		controllerName := daemonSetInfo.ControllerName
		controllerInfo, ok := controllersByName[controllerName]
		if !ok {
			continue
		}

		appliedVersion, hasAppliedVersion := getAppliedControllerVersion(input, controllerName)
		desiredStatus, shouldSetAppliedVersion := calculateDesiredControllerStatus(daemonSetInfo, controllerInfo, appliedVersion, hasAppliedVersion)
		if shouldSetAppliedVersion {
			setAppliedControllerVersion(input, controllerName, desiredStatus.Version)
		}

		patchStatus := buildControllerStatusPatch(controllerInfo, desiredStatus, now)
		if patchStatus == nil {
			continue
		}

		patchIngressNginxControllerStatus(input, controllerName, patchStatus)
	}

	return nil
}
