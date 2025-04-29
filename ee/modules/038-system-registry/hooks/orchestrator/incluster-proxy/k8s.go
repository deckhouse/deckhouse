/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	VersionAnnotation = "registry.deckhouse.io/incluster-proxy-version"
	DeploymentName    = "registry-incluster-proxy"
)

func KubernetesConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "apps/v1",
		Kind:              "Deployment",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{DeploymentName},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var d appsv1.Deployment
			err := sdk.FromUnstructured(obj, &d)
			if err != nil {
				return nil, fmt.Errorf("failed to convert deployment \"%s\" to struct: %v", obj.GetName(), err)
			}

			readyMsg, isReady := assessDeploymentStatus(&d)
			ret := Inputs{
				IsExist:  true,
				IsReady:  isReady,
				ReadyMsg: readyMsg,
				Version:  d.Annotations[VersionAnnotation],
			}
			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	return helpers.SnapshotToSingle[Inputs](input, name)
}

// assessDeploymentStatus evaluates whether a Deployment has reached its desired state.
// It checks the status fields to determine if the number of updated, available, and total replicas
// match the expected specification.

// Deployment status fields explanation:
// - status.replicas:
//     Total number of non-terminated Pods (Pending or Running) across all ReplicaSets
// - status.availableReplicas:
//     Number of Pods that have been available for at least minReadySeconds across all ReplicaSets
// - status.unavailableReplicas:
//     Difference between status.replicas and status.availableReplicas across all ReplicaSets
// - status.readyReplicas:
//     Number of Pods in Ready condition (may not be Available yet) across all ReplicaSets
// - status.updatedReplicas:
//     Number of Pods created using the latest Deployment template (podTemplateSpec)

func assessDeploymentStatus(deployment *appsv1.Deployment) (string, bool) {
	// Check if the Deployment controller has observed the latest desired spec
	if deployment.Generation > deployment.Status.ObservedGeneration {
		return "Deployment update is not yet observed by the controller", false
	}

	// Default replicas to 1 if unspecified
	// From spec:
	// 		Number of desired pods. This is a pointer to distinguish between explicit
	// 		zero and not specified. Defaults to 1.
	var desiredReplicas int32 = 1
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}

	// Condition 1: Updated replicas must match the desired count
	if deployment.Status.UpdatedReplicas < desiredReplicas {
		msg := fmt.Sprintf(
			"Deployment %q: %d of %d Pods have been updated to the latest specification",
			deployment.Name,
			deployment.Status.UpdatedReplicas,
			desiredReplicas,
		)
		return msg, false
	}

	// Condition 2: All old replicas should be terminated
	if deployment.Status.UpdatedReplicas < deployment.Status.Replicas {
		msg := fmt.Sprintf(
			"Deployment %q: %d outdated Pods are still running",
			deployment.Name,
			deployment.Status.Replicas-deployment.Status.UpdatedReplicas,
		)
		return msg, false
	}

	// Condition 3: All updated replicas must become available
	if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
		msg := fmt.Sprintf(
			"Deployment %q: %d of %d updated Pods are currently available",
			deployment.Name,
			deployment.Status.AvailableReplicas,
			deployment.Status.UpdatedReplicas,
		)
		return msg, false
	}

	// Deployment matches the desired specification
	return fmt.Sprintf("Deployment %q is in the desired state", deployment.Name), true
}
