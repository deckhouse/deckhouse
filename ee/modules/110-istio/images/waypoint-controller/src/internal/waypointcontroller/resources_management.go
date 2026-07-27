/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"fmt"
	"math"
	"strings"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

const (
	defaultProxyEphemeralStorageRequest = "50Mi"
	defaultStaticCPURequest             = "100m"
	defaultStaticMemoryRequest          = "128Mi"
	defaultVPACPUMin                    = "100m"
	defaultVPACPUMax                    = "1000m"
	defaultVPAMemoryMin                 = "128Mi"
	defaultVPAMemoryMax                 = "2000Mi"
	waypointProxyContainerName          = "istio-proxy"
)

func newResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
}

func resourcesFromResourcesManagement(spec *networkv1alpha1.ResourcesManagement) (corev1.ResourceRequirements, error) {
	switch {
	case spec == nil:
		return resourcesFromResourcesManagementVPA(nil)
	case spec.Mode == "Static":
		return resourcesFromResourcesManagementStatic(spec)
	case spec.Mode == "VPA", spec.Mode == "":
		return resourcesFromResourcesManagementVPA(spec)
	default:
		return corev1.ResourceRequirements{}, fmt.Errorf("unknown resourcesManagement.mode: %q", spec.Mode)
	}
}

func resourcesFromResourcesManagementStatic(spec *networkv1alpha1.ResourcesManagement) (corev1.ResourceRequirements, error) {
	reqs := newResourceRequirements()

	cpuRequest := defaultStaticCPURequest
	memoryRequest := defaultStaticMemoryRequest
	if spec != nil && spec.Static != nil && spec.Static.Requests != nil {
		if strings.TrimSpace(spec.Static.Requests.CPU) != "" {
			cpuRequest = spec.Static.Requests.CPU
		}
		if strings.TrimSpace(spec.Static.Requests.Memory) != "" {
			memoryRequest = spec.Static.Requests.Memory
		}
	}

	if err := setResourceListQuantity(reqs.Requests, corev1.ResourceCPU, cpuRequest, "resourcesManagement.static.requests.cpu"); err != nil {
		return corev1.ResourceRequirements{}, err
	}
	if err := setResourceListQuantity(reqs.Requests, corev1.ResourceMemory, memoryRequest, "resourcesManagement.static.requests.memory"); err != nil {
		return corev1.ResourceRequirements{}, err
	}

	if spec != nil && spec.Static != nil && spec.Static.Limits != nil {
		if err := setResourceListQuantity(reqs.Limits, corev1.ResourceCPU, spec.Static.Limits.CPU, "resourcesManagement.static.limits.cpu"); err != nil {
			return corev1.ResourceRequirements{}, err
		}
		if err := setResourceListQuantity(reqs.Limits, corev1.ResourceMemory, spec.Static.Limits.Memory, "resourcesManagement.static.limits.memory"); err != nil {
			return corev1.ResourceRequirements{}, err
		}
	}

	if err := finalizeResourceRequirements(&reqs); err != nil {
		return corev1.ResourceRequirements{}, err
	}

	return reqs, nil
}

func resourcesFromResourcesManagementVPA(spec *networkv1alpha1.ResourcesManagement) (corev1.ResourceRequirements, error) {
	reqs := newResourceRequirements()

	cpuMin, err := quantityForVPAResource(spec, true, true)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	memoryMin, err := quantityForVPAResource(spec, false, true)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}

	reqs.Requests[corev1.ResourceCPU] = cpuMin
	reqs.Requests[corev1.ResourceMemory] = memoryMin

	if spec != nil && spec.VPA != nil {
		if spec.VPA.CPU != nil && spec.VPA.CPU.LimitRatio != nil && *spec.VPA.CPU.LimitRatio > 0 {
			reqs.Limits[corev1.ResourceCPU] = quantityFromRatio(cpuMin, *spec.VPA.CPU.LimitRatio, corev1.ResourceCPU)
		}
		if spec.VPA.Memory != nil && spec.VPA.Memory.LimitRatio != nil && *spec.VPA.Memory.LimitRatio > 0 {
			reqs.Limits[corev1.ResourceMemory] = quantityFromRatio(memoryMin, *spec.VPA.Memory.LimitRatio, corev1.ResourceMemory)
		}
	}

	if err := finalizeResourceRequirements(&reqs); err != nil {
		return corev1.ResourceRequirements{}, err
	}

	return reqs, nil
}

func newVPAForWaypoint(instance *networkv1alpha1.WaypointInstance) (*vpav1.VerticalPodAutoscaler, error) {
	policies, err := vpaContainerResourcePolicies(instance.Spec.ResourcesManagement)
	if err != nil {
		return nil, err
	}

	labels := instanceLabels(instance)
	labels[WaypointComponentLabelKey] = "vpa"

	vpa := &vpav1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceBaseName(instance.Name),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: vpav1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       resourceBaseName(instance.Name),
			},
			UpdatePolicy: &vpav1.PodUpdatePolicy{
				UpdateMode: vpaUpdateMode(instance.Spec.ResourcesManagement),
			},
		},
	}

	if len(policies) > 0 {
		vpa.Spec.ResourcePolicy = &vpav1.PodResourcePolicy{
			ContainerPolicies: policies,
		}
	}

	return vpa, nil
}

func vpaContainerResourcePolicies(spec *networkv1alpha1.ResourcesManagement) ([]vpav1.ContainerResourcePolicy, error) {
	policy := vpav1.ContainerResourcePolicy{
		ContainerName:    waypointProxyContainerName,
		MinAllowed:       corev1.ResourceList{},
		MaxAllowed:       corev1.ResourceList{},
		ControlledValues: ptr.To(vpav1.ContainerControlledValuesRequestsAndLimits),
	}

	minCPU, err := quantityForVPAResource(spec, true, true)
	if err != nil {
		return nil, err
	}
	maxCPU, err := quantityForVPAResource(spec, true, false)
	if err != nil {
		return nil, err
	}
	minMemory, err := quantityForVPAResource(spec, false, true)
	if err != nil {
		return nil, err
	}
	maxMemory, err := quantityForVPAResource(spec, false, false)
	if err != nil {
		return nil, err
	}

	policy.MinAllowed[corev1.ResourceCPU] = minCPU
	policy.MaxAllowed[corev1.ResourceCPU] = maxCPU
	policy.MinAllowed[corev1.ResourceMemory] = minMemory
	policy.MaxAllowed[corev1.ResourceMemory] = maxMemory

	return []vpav1.ContainerResourcePolicy{policy}, nil
}

func vpaUpdateMode(spec *networkv1alpha1.ResourcesManagement) *vpav1.UpdateMode {
	if spec == nil || spec.VPA == nil || spec.VPA.Mode == "" {
		return ptr.To(vpav1.UpdateModeInPlaceOrRecreate)
	}

	switch spec.VPA.Mode {
	case "Initial":
		return ptr.To(vpav1.UpdateModeInitial)
	case "InPlaceOrRecreate":
		return ptr.To(vpav1.UpdateModeInPlaceOrRecreate)
	default:
		return ptr.To(vpav1.UpdateModeInPlaceOrRecreate)
	}
}

func parseOptionalQuantity(value, fieldPath string) (*resource.Quantity, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	q, err := resource.ParseQuantity(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fieldPath, err)
	}

	return &q, nil
}

func setResourceListQuantity(dst corev1.ResourceList, name corev1.ResourceName, value, fieldPath string) error {
	q, err := parseOptionalQuantity(value, fieldPath)
	if err != nil {
		return err
	}
	if q != nil {
		dst[name] = *q
	}

	return nil
}

func quantityFromRatio(base resource.Quantity, ratio float64, resourceName corev1.ResourceName) resource.Quantity {
	switch resourceName {
	case corev1.ResourceCPU:
		return *resource.NewMilliQuantity(int64(math.Ceil(float64(base.MilliValue())*ratio)), resource.DecimalSI)
	case corev1.ResourceMemory:
		return *resource.NewQuantity(int64(math.Ceil(float64(base.Value())*ratio)), resource.BinarySI)
	default:
		return resource.Quantity{}
	}
}

func finalizeResourceRequirements(reqs *corev1.ResourceRequirements) error {
	if err := setDefaultEphemeralStorageRequest(reqs); err != nil {
		return err
	}
	if len(reqs.Limits) == 0 {
		reqs.Limits = nil
	}

	return nil
}

func setDefaultEphemeralStorageRequest(reqs *corev1.ResourceRequirements) error {
	if reqs == nil {
		return nil
	}
	if reqs.Requests == nil {
		reqs.Requests = corev1.ResourceList{}
	}

	q, err := resource.ParseQuantity(defaultProxyEphemeralStorageRequest)
	if err != nil {
		return fmt.Errorf("invalid default ephemeral-storage request %q: %w", defaultProxyEphemeralStorageRequest, err)
	}
	reqs.Requests[corev1.ResourceEphemeralStorage] = q

	return nil
}

func quantityForVPAResource(spec *networkv1alpha1.ResourcesManagement, isCPU bool, isMin bool) (resource.Quantity, error) {
	value := defaultVPAResourceValue(isCPU, isMin)
	if spec != nil && spec.VPA != nil {
		var source *networkv1alpha1.VPAResource
		if isCPU {
			source = spec.VPA.CPU
		} else {
			source = spec.VPA.Memory
		}
		if source != nil {
			if isMin && strings.TrimSpace(source.Min) != "" {
				value = source.Min
			}
			if !isMin && strings.TrimSpace(source.Max) != "" {
				value = source.Max
			}
		}
	}

	fieldPath := "resourcesManagement.vpa.memory.min"
	if isCPU && isMin {
		fieldPath = "resourcesManagement.vpa.cpu.min"
	}
	if isCPU && !isMin {
		fieldPath = "resourcesManagement.vpa.cpu.max"
	}
	if !isCPU && !isMin {
		fieldPath = "resourcesManagement.vpa.memory.max"
	}

	q, err := parseOptionalQuantity(value, fieldPath)
	if err != nil {
		return resource.Quantity{}, err
	}
	if q == nil {
		value = defaultVPAResourceValue(isCPU, isMin)
		q, err = parseOptionalQuantity(value, fieldPath)
		if err != nil {
			return resource.Quantity{}, err
		}
	}

	return *q, nil
}

func defaultVPAResourceValue(isCPU bool, isMin bool) string {
	switch {
	case isCPU && isMin:
		return defaultVPACPUMin
	case isCPU && !isMin:
		return defaultVPACPUMax
	case !isCPU && isMin:
		return defaultVPAMemoryMin
	default:
		return defaultVPAMemoryMax
	}
}
