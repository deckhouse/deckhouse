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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

type waypointPodSpecConfig struct {
	InstanceName          string
	Namespace             string
	ClusterDomain         string
	ProxyImage            string
	Resources             corev1.ResourceRequirements
	ServiceAccount        string
	NodeSelector          map[string]string
	Tolerations           []corev1.Toleration
	IstioRevision         string
	IstioNetworkName      string
	IstioCloudPlatform    string
	IstioClusterID        string
	EnablePodAntiAffinity bool
}

func waypointPodSpec(cfg waypointPodSpecConfig) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{
		ServiceAccountName:            cfg.ServiceAccount,
		ImagePullSecrets:              []corev1.LocalObjectReference{{Name: "d8-istio-sidecar-registry"}},
		TerminationGracePeriodSeconds: ptr.To(int64(2)),
	}

	if len(cfg.NodeSelector) > 0 {
		podSpec.NodeSelector = cfg.NodeSelector
	}

	if len(cfg.Tolerations) > 0 {
		podSpec.Tolerations = append(podSpec.Tolerations, cfg.Tolerations...)
	}

	if cfg.EnablePodAntiAffinity {
		podSpec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									AppLabelKey:                              AppLabelValue,
									"gateway.networking.k8s.io/gateway-name": ResourceNamePrefix + cfg.InstanceName,
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		}
	}

	container, err := istioProxyContainer(cfg)
	if err != nil {
		return corev1.PodSpec{}, err
	}
	podSpec.Containers = []corev1.Container{container}
	podSpec.Volumes = WaypointVolumes()
	podSpec.InitContainers = nil

	return podSpec, nil
}

func istioProxyContainer(cfg waypointPodSpecConfig) (corev1.Container, error) {
	env, err := WaypointEnv(cfg)
	if err != nil {
		return corev1.Container{}, err
	}

	container := corev1.Container{
		Name:            "istio-proxy",
		Image:           cfg.ProxyImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"proxy",
			"waypoint",
			"--domain", "$(POD_NAMESPACE).svc." + cfg.ClusterDomain,
			"--serviceCluster", "d8-waypoint-" + cfg.InstanceName + ".$(POD_NAMESPACE)",
			"--proxyLogLevel", "warning",
			"--proxyComponentLogLevel", "misc:error",
			"--log_output_level", "default:info",
		},
		Ports: []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 15020, Protocol: corev1.ProtocolTCP},
			{Name: "status-port", ContainerPort: 15021, Protocol: corev1.ProtocolTCP},
			{Name: "http-envoy-prom", ContainerPort: 15090, Protocol: corev1.ProtocolTCP},
		},
		ReadinessProbe: &corev1.Probe{
			FailureThreshold: 4,
			PeriodSeconds:    15,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz/ready",
					Port:   intstr.FromInt(15021),
					Scheme: corev1.URISchemeHTTP,
				},
			},
		},
		StartupProbe: &corev1.Probe{
			FailureThreshold:    30,
			InitialDelaySeconds: 1,
			PeriodSeconds:       1,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz/ready",
					Port:   intstr.FromInt(15021),
					Scheme: corev1.URISchemeHTTP,
				},
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			Privileged:             ptr.To(false),
			ReadOnlyRootFilesystem: ptr.To(true),
			RunAsGroup:             ptr.To(int64(1337)),
			RunAsNonRoot:           ptr.To(true),
			RunAsUser:              ptr.To(int64(1337)),
		},
		Resources:    cfg.Resources,
		Env:          env,
		VolumeMounts: WaypointVolumeMounts(),
	}

	return container, nil
}
