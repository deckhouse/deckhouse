package resources

import (
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

// ControllerMustHaveVPA fills linting error regarding VPA
func ControllerMustHaveVPA(module types.Module, values string, objectStore *storage.UnstructuredObjectStore, lintRuleErrorsList *errors.LintRuleErrorsList) {
	if !isVPAEnabled(values) {
		return
	}

	scope := newLintingScope(objectStore, lintRuleErrorsList)

	vpaTargets, vpaTolerationGroups := parseTargetsAndTolerationGroups(scope)

	for index, object := range scope.Objects() {
		// Skip non-pod controllers and modules which control VPA themselves
		if !isPodController(object.Unstructured.GetKind()) || shouldSkipModuleResource(module.Name, &index) {
			continue
		}

		if !ensureVPAIsPresent(scope, vpaTargets, index, object) {
			continue
		}

		if !ensureContainersWithoutRequests(scope, object) {
			continue
		}

		ensureTolerations(scope, vpaTolerationGroups, index, object)
	}
}

func isVPAEnabled(values string) bool {
	var v struct {
		Global struct{ EnabledModules []string }
	}
	err := yaml.Unmarshal([]byte(values), &v)
	if err != nil {
		panic("unable to parse global.enabledModules values section")
	}

	for _, module := range v.Global.EnabledModules {
		if module == "vertical-pod-autoscaler-crd" {
			return true
		}
	}
	return false
}

func isPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}

func shouldSkipModuleResource(moduleName string, r *storage.ResourceIndex) bool {
	switch moduleName {
	// Controllers VPA is configured through cr settings
	case "ingress-nginx":
		return r.Kind == "DaemonSet" && r.Namespace == "d8-ingress-nginx" && strings.HasPrefix(r.Name, "controller-")

	// Network gateway snat daemonset tolerations is configured through module values
	case "network-gateway":
		return r.Kind == "DaemonSet" && r.Namespace == "d8-network-gateway" && r.Name == "snat"

	// Metal LB speaker daemonset tolerations is configured through module values
	case "metallb":
		return r.Kind == "DaemonSet" && r.Namespace == "d8-metallb" && r.Name == "speaker"

	default:
		return false
	}
}

// parseTargetsAndTolerationGroups resolves target resource indexes
func parseTargetsAndTolerationGroups(scope *lintingScope) (map[storage.ResourceIndex]struct{}, map[storage.ResourceIndex]string) {
	vpaTargets := make(map[storage.ResourceIndex]struct{})
	vpaTolerationGroups := make(map[storage.ResourceIndex]string)

	for _, object := range scope.Objects() {
		kind := object.Unstructured.GetKind()

		if kind != "VerticalPodAutoscaler" {
			continue
		}

		fillVPAMaps(scope, vpaTargets, vpaTolerationGroups, object)

	}

	return vpaTargets, vpaTolerationGroups
}

func fillVPAMaps(scope *lintingScope, vpaTargets map[storage.ResourceIndex]struct{}, vpaTolerationGroups map[storage.ResourceIndex]string, vpa storage.StoreObject) {
	target, ok := parseVPATargetIndex(scope, vpa)
	if !ok {
		return
	}

	vpaTargets[target] = struct{}{}

	labels := vpa.Unstructured.GetLabels()
	if label, ok := labels["workload-resource-policy.deckhouse.io"]; ok {
		vpaTolerationGroups[target] = label
	}
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

// ensureContainersWithoutRequests verifies containers don't have their own requests, adds linting error otherwise
// returns true if linting passed, otherwise returns false
func ensureContainersWithoutRequests(scope *lintingScope, object storage.StoreObject) bool {
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

	for _, container := range containers {
		res := container.Resources.Requests
		if res.Cpu().IsZero() && res.Memory().IsZero() {
			continue
		}

		scope.AddError(
			"VPA005",
			object.Identity()+"; container = "+container.Name,
			fmt.Sprintf("cpu = %s, memory = %s", res.Cpu().String(), res.Memory().String()),
			"The container must not have resources requests, because resources are managed by VPA",
		)
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
		if toleration.Key == "node-role.kubernetes.io/master" || (toleration.Key == "" && toleration.Operator == "Exists") {
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
