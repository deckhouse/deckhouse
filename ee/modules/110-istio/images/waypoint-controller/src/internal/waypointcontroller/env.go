/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func WaypointEnv(cfg waypointPodSpecConfig) ([]corev1.EnvVar, error) {
	envs := []corev1.EnvVar{}

	envs = append(envs, corev1.EnvVar{
		Name: "ISTIO_META_SERVICE_ACCOUNT",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.serviceAccountName",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "ISTIO_META_NODE_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.nodeName",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{Name: "PILOT_CERT_PROVIDER", Value: "istiod"})
	envs = append(envs, corev1.EnvVar{Name: "CA_ADDR", Value: fmt.Sprintf("istiod-%s.d8-istio.svc:15012", cfg.IstioRevision)})

	envs = append(envs, corev1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "POD_NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "INSTANCE_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "status.podIP",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "SERVICE_ACCOUNT",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.serviceAccountName",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "HOST_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "status.hostIP",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "ISTIO_CPU_LIMIT",
		ValueFrom: &corev1.EnvVarSource{
			ResourceFieldRef: &corev1.ResourceFieldSelector{
				Resource: "limits.cpu",
			},
		},
	})

	discoveryAddr := fmt.Sprintf("istiod-%s.d8-istio.svc:15012", cfg.IstioRevision)

	// Normalize cloud platform: fall back to "none" if empty.
	cloudPlatform := cfg.IstioCloudPlatform
	if cloudPlatform == "" {
		cloudPlatform = "none"
	}

	proxyConfigJSON, err := json.Marshal(map[string]interface{}{
		"discoveryAddress":                discoveryAddr,
		"holdApplicationUntilProxyStarts": false,
		"meshId":                          "d8-istio-mesh",
		"proxyMetadata": map[string]string{
			"CLOUD_PLATFORM":               cloudPlatform,
			"ISTIO_META_DNS_AUTO_ALLOCATE": "true",
			"ISTIO_META_DNS_CAPTURE":       "true",
			"ISTIO_META_ENABLE_HBONE":      "true",
			"ISTIO_META_IDLE_TIMEOUT":      "1h",
			"PROXY_CONFIG_XDS_AGENT":       "true",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal proxy config: %w", err)
	}

	envs = append(envs, corev1.EnvVar{Name: "PROXY_CONFIG", Value: string(proxyConfigJSON)})
	envs = append(envs, corev1.EnvVar{Name: "CLOUD_PLATFORM", Value: cloudPlatform})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_DNS_AUTO_ALLOCATE", Value: "true"})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_DNS_CAPTURE", Value: "true"})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_ENABLE_HBONE", Value: "true"})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_IDLE_TIMEOUT", Value: "1h"})
	envs = append(envs, corev1.EnvVar{Name: "PROXY_CONFIG_XDS_AGENT", Value: "true"})

	envs = append(envs, corev1.EnvVar{
		Name: "GOMEMLIMIT",
		ValueFrom: &corev1.EnvVarSource{
			ResourceFieldRef: &corev1.ResourceFieldSelector{
				Resource: "limits.memory",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{
		Name: "GOMAXPROCS",
		ValueFrom: &corev1.EnvVarSource{
			ResourceFieldRef: &corev1.ResourceFieldSelector{
				Resource: "limits.cpu",
			},
		},
	})

	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_CLUSTER_ID", Value: cfg.IstioClusterID})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_NETWORK", Value: cfg.IstioNetworkName})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_INTERCEPTION_MODE", Value: "REDIRECT"})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_WORKLOAD_NAME", Value: "d8-waypoint-" + cfg.InstanceName})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_OWNER", Value: fmt.Sprintf("kubernetes://apis/apps/v1/namespaces/%s/deployments/d8-waypoint-%s", cfg.Namespace, cfg.InstanceName)})
	envs = append(envs, corev1.EnvVar{Name: "ISTIO_META_MESH_ID", Value: "d8-istio-mesh"})
	envs = append(envs, corev1.EnvVar{Name: "TRUST_DOMAIN", Value: cfg.ClusterDomain})

	return envs, nil
}
