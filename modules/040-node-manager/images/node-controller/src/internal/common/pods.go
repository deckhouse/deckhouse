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

package common

import corev1 "k8s.io/api/core/v1"

// APIServerPodLabels selects the kube-apiserver Pods of the control plane. The manager's cache
// filter and every reader share this set: one that drifts matches nothing without an error, and
// for the NodeConfig and bashible readers that reaches every node.
var APIServerPodLabels = map[string]string{
	"component": "kube-apiserver",
	"tier":      "control-plane",
}

// IsAPIServerPod reports whether an object is one of those Pods.
func IsAPIServerPod(object interface{ GetLabels() map[string]string }) bool {
	labels := object.GetLabels()
	for key, value := range APIServerPodLabels {
		if labels[key] != value {
			return false
		}
	}
	return true
}

// PodReady reports the Ready condition of a Pod.
func PodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
