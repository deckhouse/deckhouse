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

package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func WorkloadLabels(appName, name string) map[string]string {
	return map[string]string{
		"app":  appName,
		"name": name,
	}
}

func SplitWorkloads(workloads []DaemonWorkload) (DaemonWorkload, DaemonWorkload) {
	var native DaemonWorkload
	var legacy DaemonWorkload

	for _, workload := range workloads {
		switch workload.(type) {
		case NativeDaemonSet:
			if native == nil {
				native = workload
			}
		case AdvancedDaemonSet:
			if legacy == nil {
				legacy = workload
			}
		}
	}

	return native, legacy
}

func SelectReadyCheckWorkload(workloads []DaemonWorkload) DaemonWorkload {
	native, legacy := SplitWorkloads(workloads)
	if native != nil {
		return native
	}

	if legacy != nil {
		return legacy
	}

	return nil
}

func SplitPods(pods []corev1.Pod) ([]corev1.Pod, []corev1.Pod) {
	var legacy []corev1.Pod
	var native []corev1.Pod

	for i := range pods {
		pod := pods[i]
		if pod.DeletionTimestamp != nil {
			continue
		}

		if !IsLegacyDaemonSetPod(pod) {
			native = append(native, pod)
			continue
		}

		legacy = append(legacy, pod)
	}

	return legacy, native
}

func IsLegacyDaemonSetPod(pod corev1.Pod) bool {
	if pod.Labels["ingress-nginx.deckhouse.io/workload-kind"] == "native" {
		return false
	}

	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" && ownerRef.APIVersion == appsv1.SchemeGroupVersion.String() {
			return false
		}
	}

	return true
}
