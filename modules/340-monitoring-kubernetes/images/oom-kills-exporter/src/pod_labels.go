// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func buildContainerLabelsFromPod(pod *corev1.Pod, containerName string) map[string]string {
	return map[string]string{
		"io.kubernetes.container.name": containerName,
		"io.kubernetes.pod.namespace":  pod.Namespace,
		"io.kubernetes.pod.uid":        string(pod.UID),
		"io.kubernetes.pod.name":       pod.Name,
	}
}

func getPodContainerIDs(pod *corev1.Pod) map[string]string {
	containerIDs := make(map[string]string)
	for _, status := range pod.Status.ContainerStatuses {
		addContainerStatus(containerIDs, status)
	}
	for _, status := range pod.Status.InitContainerStatuses {
		addContainerStatus(containerIDs, status)
	}
	for _, status := range pod.Status.EphemeralContainerStatuses {
		addContainerStatus(containerIDs, status)
	}
	return containerIDs
}

func addContainerStatus(containerIDs map[string]string, status corev1.ContainerStatus) {
	if status.ContainerID == "" {
		return
	}
	containerID := normalizeContainerID(status.ContainerID)
	if containerID == "" {
		return
	}
	containerIDs[containerID] = status.Name
}

func normalizeContainerID(containerID string) string {
	if idx := strings.Index(containerID, "://"); idx >= 0 {
		return containerID[idx+3:]
	}
	return containerID
}
