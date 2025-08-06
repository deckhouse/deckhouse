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

package helpers

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
)

// AssessDeploymentStatus evaluates whether a Deployment has reached its desired state.
func AssessDeploymentStatus(deployment *appsv1.Deployment) (string, bool) {
	// Check if the Deployment controller has observed the latest desired spec
	if deployment.Generation > deployment.Status.ObservedGeneration {
		return "Deployment update is not yet observed by the controller", false
	}

	// Default replicas to 1 if unspecified
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
