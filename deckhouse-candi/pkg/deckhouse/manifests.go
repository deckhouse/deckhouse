package deckhouse

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	deckhouseRegistrySecretName = "deckhouse-registry"
	deckhouseRegistryVolumeName = "registrysecret"
)

//nolint:funlen
func generateDeckhouseDeployment(registry, logLevel, bundle string, isSecureRegistry bool) *appsv1.Deployment {
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse",
			Namespace: "d8-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1), //nolint:gomnd
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deckhouse",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "deckhouse",
					},
				},
				Spec: apiv1.PodSpec{
					HostNetwork:        true,
					DNSPolicy:          apiv1.DNSDefault,
					ServiceAccountName: "deckhouse",
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
					Tolerations: []apiv1.Toleration{
						{Operator: apiv1.TolerationOpExists},
					},
					Containers: []apiv1.Container{
						{
							Name:            "deckhouse",
							Image:           registry,
							Command:         []string{"/deckhouse/deckhouse"},
							ImagePullPolicy: apiv1.PullAlways,
							Ports: []apiv1.ContainerPort{
								{
									ContainerPort: 9650,
								},
							},
							Resources: apiv1.ResourceRequirements{
								Requests: map[apiv1.ResourceName]resource.Quantity{
									apiv1.ResourceCPU:    resource.MustParse("50m"),
									apiv1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							Env: []apiv1.EnvVar{
								{
									Name:  "RLOG_LOG_LEVEL",
									Value: logLevel,
								},
								{
									Name:  "DECKHOUSE_BUNDLE",
									Value: bundle,
								},
								{
									Name: "DECKHOUSE_POD",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "HELM_HOST",
									Value: "127.0.0.1:44434",
								},
								{
									Name:  "ADDON_OPERATOR_CONFIG_MAP",
									Value: "deckhouse",
								},
								{
									Name:  "ADDON_OPERATOR_PROMETHEUS_METRICS_PREFIX",
									Value: "deckhouse_",
								},
								{
									Name: "ADDON_OPERATOR_NAMESPACE",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "ADDON_OPERATOR_LISTEN_ADDRESS",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name:  "KUBERNETES_DEPLOYED",
									Value: time.Unix(0, time.Now().Unix()).String(),
								},
							},
							WorkingDir: "/deckhouse",
						},
					},
				},
			},
		},
	}

	if isSecureRegistry {
		deployment.Spec.Template.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{
			{Name: deckhouseRegistrySecretName},
		}

		deployment.Spec.Template.Spec.Volumes = []apiv1.Volume{
			{
				Name: deckhouseRegistryVolumeName,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{SecretName: deckhouseRegistrySecretName},
				},
			},
		}

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []apiv1.VolumeMount{
			{
				Name:      deckhouseRegistryVolumeName,
				MountPath: "/etc/registrysecret",
				ReadOnly:  true,
			},
		}
	}
	return &deployment
}

func generateDeckhouseNamespace(name string) *apiv1.Namespace {
	return &apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
			Annotations: map[string]string{
				"extended-monitoring.flant.com/enabled": "",
			},
		},
		Spec: apiv1.NamespaceSpec{
			Finalizers: []apiv1.FinalizerName{
				apiv1.FinalizerKubernetes,
			},
		},
	}
}

func generateDeckhouseServiceAccount() *apiv1.ServiceAccount {
	return &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
	}
}

func generateDeckhouseAdminClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-admin",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{rbacv1.VerbAll},
			},
			{
				NonResourceURLs: []string{rbacv1.NonResourceAll},
				Verbs:           []string{rbacv1.VerbAll},
			},
		},
	}
}

func generateDeckhouseAdminClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "deckhouse",
				Namespace: "d8-system",
			},
		},
	}
}

func generateDeckhouseRegistrySecret(dockerCfg string) *apiv1.Secret {
	data, _ := base64.StdEncoding.DecodeString(dockerCfg)
	return &apiv1.Secret{
		Type: apiv1.SecretTypeDockercfg,
		ObjectMeta: metav1.ObjectMeta{
			Name: deckhouseRegistrySecretName,
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Data: map[string][]byte{
			apiv1.DockerConfigKey: data,
		},
	}
}

func generateDeckhouseConfigMap(deckhouseConfig map[string]interface{}) (*apiv1.ConfigMap, error) {
	configMap := apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
	}

	configMapData := make(map[string]string, len(deckhouseConfig))
	for setting, data := range deckhouseConfig {
		if strings.HasSuffix(setting, "Enabled") {
			boolData, ok := data.(bool)
			if !ok {
				return nil, fmt.Errorf("deckhouse config map: %q must be bool", setting)
			}
			configMapData[setting] = strconv.FormatBool(boolData)

			continue
		}
		convertedData, err := yaml.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("preparing deckhouse config map error: %v", err)
		}
		configMapData[setting] = string(convertedData)
	}
	configMap.Data = configMapData

	return &configMap, nil
}

func generateSecret(name, namespace string, data map[string][]byte) *apiv1.Secret {
	return &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Data: data,
	}
}

func generateSecretWithTerraformState(data []byte) *apiv1.Secret {
	return generateSecret(
		"d8-cluster-teraform-state",
		"kube-system",
		map[string][]byte{
			"cluster_terraform_state.json": data,
		},
	)
}

func generateSecretWithClusterConfig(data []byte) *apiv1.Secret {
	return generateSecret("d8-cluster-configuration", "kube-system",
		map[string][]byte{"cluster-configuration.yaml": data})
}

func generateSecretWithProviderClusterConfig(configData, discoveryData []byte) *apiv1.Secret {
	return generateSecret("d8-provider-cluster-configuration", "kube-system",
		map[string][]byte{
			"cloud-provider-cluster-configuration.yaml": configData,
			"cloud-provider-discovery-data.json":        discoveryData,
		})
}

func int32Ptr(i int32) *int32 { return &i }
