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

package podstatus

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
)

var problematicStatuses = []string{"Failed", "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "Error", "Evicted"}

func IsHealthy(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && slices.Contains(problematicStatuses, containerStatus.State.Waiting.Reason) ||
			containerStatus.State.Terminated != nil && slices.Contains(problematicStatuses, containerStatus.State.Terminated.Reason) {
			return false
		}
	}

	return true
}

func GetProblematicStatuses() []string {
	return problematicStatuses
}
