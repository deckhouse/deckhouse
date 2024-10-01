/*
Copyright 2021 Flant JSC

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

package resources

import (
	"fmt"

	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

// ControllerMustHaveVPA fills linting error regarding VPA
func ControllerMustHaveVPA(linter *rules.ObjectLinter) {
	if !linter.EnabledModules.Has("vertical-pod-autoscaler") {
		return
	}

	scope := newLintingScope(linter.ObjectStore, linter.ErrorsList)

	vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes := parseTargetsAndTolerationGroups(scope)

	for index, object := range scope.Objects() {
		// Skip non-pod controllers
		if !isPodController(object.Unstructured.GetKind()) {
			continue
		}

		if !ensureVPAIsPresent(scope, vpaTargets, index, object) {
			continue
		}

		// for vpa UpdateMode Off we cannot have container resource policies in vpa object
		if vpaUpdateModes[index] == utils.UpdateModeOff {
			continue
		}

		if !ensureVPAContainersMatchControllerContainers(scope, object, index, vpaContainerNamesMap) {
			continue
		}

		ensureTolerations(scope, vpaTolerationGroups, index, object)
	}
}

func isPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}

func isPodControllerDaemonSet(kind string) bool {
	return kind == "DaemonSet"
}

// parseTargetsAndTolerationGroups resolves target resource indexes
func parseTargetsAndTolerationGroups(scope *lintingScope) (map[storage.ResourceIndex]struct{}, map[storage.ResourceIndex]string, map[storage.ResourceIndex]set.Set, map[storage.ResourceIndex]utils.UpdateMode) {
	vpaTargets := make(map[storage.ResourceIndex]struct{})
	vpaTolerationGroups := make(map[storage.ResourceIndex]string)
	vpaContainerNamesMap := make(map[storage.ResourceIndex]set.Set)
	vpaUpdateModes := make(map[storage.ResourceIndex]utils.UpdateMode)

	for _, object := range scope.Objects() {
		kind := object.Unstructured.GetKind()

		if kind != "VerticalPodAutoscaler" {
			continue
		}

		fillVPAMaps(scope, vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes, object)
	}

	return vpaTargets, vpaTolerationGroups, vpaContainerNamesMap, vpaUpdateModes
}

func fillVPAMaps(scope *lintingScope, vpaTargets map[storage.ResourceIndex]struct{}, vpaTolerationGroups map[storage.ResourceIndex]string, vpaContainerNamesMap map[storage.ResourceIndex]set.Set, vpaUpdateModes map[storage.ResourceIndex]utils.UpdateMode, vpa storage.StoreObject) {
	target, ok := parseVPATargetIndex(scope, vpa)
	if !ok {
		return
	}

	vpaTargets[target] = struct{}{}

	labels := vpa.Unstructured.GetLabels()
	if label, ok := labels["workload-resource-policy.deckhouse.io"]; ok {
		vpaTolerationGroups[target] = label
	}

	updateMode, vnm, ok := parseVPAResourcePolicyContainers(scope, vpa)
	if !ok {
		return
	}
	vpaContainerNamesMap[target] = vnm
	vpaUpdateModes[target] = updateMode
}

// parseVPAResourcePolicyContainers parses VPA containers names in ResourcePolicy and check if minAllowed and maxAllowed for container is set
func parseVPAResourcePolicyContainers(scope *lintingScope, vpaObject storage.StoreObject) (utils.UpdateMode, set.Set, bool) {
	containers := set.New()

	v := &utils.VerticalPodAutoscaler{}
	err := sdk.FromUnstructured(&vpaObject.Unstructured, v)

	if err != nil {
		scope.AddError("VPA005", vpaObject.Identity(), false, "Cannot unmarshal VPA object: %v", err)
		return "", containers, false
	}

	updateMode := *v.Spec.UpdatePolicy.UpdateMode
	if updateMode == utils.UpdateModeOff {
		return updateMode, containers, true
	}

	if v.Spec.ResourcePolicy == nil || len(v.Spec.ResourcePolicy.ContainerPolicies) == 0 {
		scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs resourcePolicy.containerPolicies is found for object")
		return updateMode, containers, false
	}

	for _, cp := range v.Spec.ResourcePolicy.ContainerPolicies {
		if cp.MinAllowed.Cpu().IsZero() {
			scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs minAllowed.cpu is found for container %s", cp.ContainerName)
			return updateMode, containers, false
		}

		if cp.MinAllowed.Memory().IsZero() {
			scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs minAllowed.memory is found for container %s", cp.ContainerName)
			return updateMode, containers, false
		}

		if cp.MaxAllowed.Cpu().IsZero() {
			scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs maxAllowed.cpu is found for container %s", cp.ContainerName)
			return updateMode, containers, false
		}

		if cp.MaxAllowed.Memory().IsZero() {
			scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs maxAllowed.memory is found for container %s", cp.ContainerName)
			return updateMode, containers, false
		}

		if cp.MinAllowed.Cpu().Cmp(*cp.MaxAllowed.Cpu()) > 0 {
			scope.AddError("VPA005", vpaObject.Identity(), false, "MinAllowed.cpu for container %s should be less than maxAllowed.cpu", cp.ContainerName)
			return updateMode, containers, false
		}

		if cp.MinAllowed.Memory().Cmp(*cp.MaxAllowed.Memory()) > 0 {
			scope.AddError("VPA005", vpaObject.Identity(), false, "MinAllowed.memory for container %s should be less than maxAllowed.memory", cp.ContainerName)
			return updateMode, containers, false
		}

		containers.Add(cp.ContainerName)
	}

	return updateMode, containers, true
}

// parseVPATargetIndex parses VPA target resource index, writes to the passed struct pointer
func parseVPATargetIndex(scope *lintingScope, vpaObject storage.StoreObject) (storage.ResourceIndex, bool) {
	target := storage.ResourceIndex{}

	specs, ok := vpaObject.Unstructured.Object["spec"].(map[string]interface{})
	if !ok {
		scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs is found for object")
		return target, false
	}

	targetRef, ok := specs["targetRef"].(map[string]interface{})
	if !ok {
		scope.AddError("VPA005", vpaObject.Identity(), false, "No VPA specs targetRef is found for object")
		return target, false
	}

	target.Namespace = vpaObject.Unstructured.GetNamespace()
	target.Name = targetRef["name"].(string)
	target.Kind = targetRef["kind"].(string)

	return target, true
}

// ensureVPAContainersMatchControllerContainers verifies VPA container names in resourcePolicy match corresponding controller container names
func ensureVPAContainersMatchControllerContainers(scope *lintingScope, object storage.StoreObject, index storage.ResourceIndex, vpaContainerNamesMap map[storage.ResourceIndex]set.Set) bool {
	vpaContainerNames, ok := vpaContainerNamesMap[index]
	if !ok {
		scope.AddError(
			"VPA005",
			"",
			false,
			"Getting vpa containers name list for the object failed: %v",
			index,
		)
		return false
	}

	containers, err := object.GetContainers()
	if err != nil {
		scope.AddError(
			"VPA005",
			object.Identity(),
			false,
			"Getting containers list for the object failed: %v",
			err,
		)
		return false
	}

	containerNames := set.New()
	for _, v := range containers {
		containerNames.Add(v.Name)
	}

	for k := range containerNames {
		if !vpaContainerNames.Has(k) {
			scope.AddError(
				"VPA005",
				fmt.Sprintf("%s ; container = %s", object.Identity(), k),
				false,
				"The container should have corresponding VPA resourcePolicy entry",
			)
		}
	}

	for k := range vpaContainerNames {
		if !containerNames.Has(k) {
			scope.AddError(
				"VPA005",
				object.Identity(),
				false,
				"VPA has resourcePolicy for container %s, but the controller does not have corresponding container resource entry", k,
			)
		}
	}

	return true
}

// returns true if linting passed, otherwise returns false
func ensureTolerations(scope *lintingScope, vpaTolerationGroups map[storage.ResourceIndex]string, index storage.ResourceIndex, object storage.StoreObject) {
	tolerations, err := getTolerationsList(object)

	if err != nil {
		scope.AddError(
			"VPA005",
			object.Identity(),
			false,
			"Get tolerations list for object failed: %v",
			err,
		)
	}

	isTolerationFound := false
	for _, toleration := range tolerations {
		if toleration.Key == "node-role.kubernetes.io/master" || toleration.Key == "node-role.kubernetes.io/control-plane" || (toleration.Key == "" && toleration.Operator == "Exists") {
			isTolerationFound = true
			break
		}
	}

	workloadLabelValue := vpaTolerationGroups[index]
	if isTolerationFound && workloadLabelValue != "every-node" && workloadLabelValue != "master" {
		scope.AddError(
			"VPA005",
			object.Identity(),
			workloadLabelValue,
			`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource not found`,
		)
	}

	if !isTolerationFound && workloadLabelValue != "" {
		scope.AddError(
			"VPA005",
			object.Identity(),
			workloadLabelValue,
			`Labels "workload-resource-policy.deckhouse.io" in corresponding VPA resource found, but tolerations is not right`,
		)
	}
}

// returns true if linting passed, otherwise returns false
func ensureVPAIsPresent(scope *lintingScope, vpaTargets map[storage.ResourceIndex]struct{}, index storage.ResourceIndex, object storage.StoreObject) bool {
	_, ok := vpaTargets[index]
	if !ok {
		scope.AddError(
			"VPA005",
			object.Identity(),
			false,
			"No VPA is found for object",
		)
	}
	return ok
}

func getTolerationsList(object storage.StoreObject) ([]v1.Toleration, error) {
	var tolerations []v1.Toleration
	converter := runtime.DefaultUnstructuredConverter

	switch object.Unstructured.GetKind() {
	case "Deployment":
		deployment := new(appsv1.Deployment)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return nil, err
		}
		tolerations = deployment.Spec.Template.Spec.Tolerations

	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return nil, err
		}
		tolerations = daemonset.Spec.Template.Spec.Tolerations

	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return nil, err
		}
		tolerations = statefulset.Spec.Template.Spec.Tolerations
	}

	return tolerations, nil
}
