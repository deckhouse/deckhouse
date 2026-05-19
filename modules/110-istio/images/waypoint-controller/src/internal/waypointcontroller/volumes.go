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

package waypointcontroller

import (
	corev1 "k8s.io/api/core/v1"
)

func WaypointVolumes() []corev1.Volume {
	volumes := []corev1.Volume{}

	volumes = append(volumes, corev1.Volume{
		Name: "workload-socket",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	volumes = append(volumes, corev1.Volume{
		Name: "istio-envoy",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	})

	volumes = append(volumes, corev1.Volume{
		Name: "istio-data",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	volumes = append(volumes, corev1.Volume{
		Name: "istio-podinfo",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "labels",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.labels",
						},
					},
					{
						Path: "annotations",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.annotations",
						},
					},
				},
			},
		},
	})

	volumes = append(volumes, corev1.Volume{
		Name: "istio-token",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{
					{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience:          "istio-ca",
							ExpirationSeconds: func() *int64 { e := int64(43200); return &e }(),
							Path:              "istio-token",
						},
					},
				},
			},
		},
	})

	volumes = append(volumes, corev1.Volume{
		Name: "istiod-ca-cert",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "istio-ca-root-cert",
				},
			},
		},
	})

	return volumes
}

func WaypointVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "workload-socket",
			MountPath: "/var/run/secrets/workload-spiffe-uds",
		},
		{
			Name:      "istiod-ca-cert",
			MountPath: "/var/run/secrets/istio",
		},
		{
			Name:      "istio-data",
			MountPath: "/var/lib/istio/data",
		},
		{
			Name:      "istio-envoy",
			MountPath: "/etc/istio/proxy",
		},
		{
			Name:      "istio-token",
			MountPath: "/var/run/secrets/tokens",
		},
		{
			Name:      "istio-podinfo",
			MountPath: "/etc/istio/pod",
		},
	}
}
