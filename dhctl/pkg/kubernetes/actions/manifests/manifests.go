// Copyright 2021 Flant JSC
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

package manifests

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	deckhouseRegistrySecretName = "deckhouse-registry"
	deckhouseRegistryVolumeName = "registrysecret"

	deployTimeEnvVarName        = "KUBERNETES_DEPLOYED"
	deployServiceHostEnvVarName = "KUBERNETES_SERVICE_HOST"
	deployServicePortEnvVarName = "KUBERNETES_SERVICE_PORT"
	deployTimeEnvVarFormat      = time.RFC3339

	ConvergeLabel = "dhctl.deckhouse.io/node-for-converge"
)

type DeckhouseDeploymentParams struct {
	Bundle   string
	Registry string
	LogLevel string

	DeployTime time.Time

	IsSecureRegistry   bool
	MasterNodeSelector bool
	KubeadmBootstrap   bool
}

func GetDeckhouseDeployTime(deployment *appsv1.Deployment) time.Time {
	deployTime := time.Time{}
	for i, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name != deployTimeEnvVarName {
			continue
		}

		timeAsString := deployment.Spec.Template.Spec.Containers[0].Env[i].Value
		t, err := time.Parse(deployTimeEnvVarFormat, timeAsString)
		if err == nil {
			deployTime = t
		}

		break
	}

	return deployTime
}

func ParameterizeDeckhouseDeployment(input *appsv1.Deployment, params DeckhouseDeploymentParams) *appsv1.Deployment {
	deployment := input.DeepCopy()

	deckhousePodTemplate := deployment.Spec.Template
	deckhouseContainer := deployment.Spec.Template.Spec.Containers[0]
	deckhouseContainerEnv := deckhouseContainer.Env

	freshDeployment := params.DeployTime.IsZero()

	if freshDeployment {
		params.DeployTime = time.Now()
	}

	var (
		deployTime        bool
		deployServiceHost bool
		deployServicePort bool
	)
	for _, envEntry := range deckhouseContainerEnv {
		deployTime = deployTime || envEntry.Name == deployTimeEnvVarName
		deployServiceHost = deployServiceHost || envEntry.Name == deployServiceHostEnvVarName
		deployServicePort = deployServicePort || envEntry.Name == deployServicePortEnvVarName
	}

	if !deployTime {
		deckhouseContainerEnv = append(deckhouseContainerEnv,
			apiv1.EnvVar{
				Name:  deployTimeEnvVarName,
				Value: params.DeployTime.Format(deployTimeEnvVarFormat),
			},
		)
	}

	if params.MasterNodeSelector {
		deckhousePodTemplate.Spec.NodeSelector = map[string]string{"node-role.kubernetes.io/control-plane": ""}
	}

	if params.IsSecureRegistry {
		deckhousePodTemplate.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{
			{Name: "deckhouse-registry"},
		}

		var volumeFound bool
		for _, volume := range deckhousePodTemplate.Spec.Volumes {
			volumeFound = volumeFound || volume.Name == deckhouseRegistryVolumeName
		}

		if !volumeFound {
			deckhousePodTemplate.Spec.Volumes = append(deckhousePodTemplate.Spec.Volumes, apiv1.Volume{
				Name: deckhouseRegistryVolumeName,
				VolumeSource: apiv1.VolumeSource{
					Secret: &apiv1.SecretVolumeSource{SecretName: deckhouseRegistrySecretName},
				},
			})
		}

		var volumeMountFound bool
		for _, volumeMount := range deckhouseContainer.VolumeMounts {
			volumeMountFound = volumeMountFound || volumeMount.Name == deckhouseRegistryVolumeName
		}

		if !volumeMountFound {
			deckhouseContainer.VolumeMounts = append(deckhouseContainer.VolumeMounts, apiv1.VolumeMount{
				Name:      deckhouseRegistryVolumeName,
				MountPath: "/etc/registrysecret",
				ReadOnly:  true,
			})
		}
	}

	if params.KubeadmBootstrap && freshDeployment {
		if !deployServiceHost {
			deckhouseContainerEnv = append(deckhouseContainerEnv,
				apiv1.EnvVar{
					Name: deployServiceHostEnvVarName,
					ValueFrom: &apiv1.EnvVarSource{
						FieldRef: &apiv1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "status.hostIP"},
					},
				},
			)
		}
		if !deployServicePort {
			deckhouseContainerEnv = append(deckhouseContainerEnv,
				apiv1.EnvVar{
					Name:  deployServicePortEnvVarName,
					Value: "6443",
				},
			)
		}
	}

	deckhouseContainer.Env = deckhouseContainerEnv
	deckhousePodTemplate.Spec.Containers = []apiv1.Container{deckhouseContainer}
	deployment.Spec.Template = deckhousePodTemplate

	return deployment
}

func DeckhouseDeployment(params DeckhouseDeploymentParams) *appsv1.Deployment {
	deckhouseDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse",
			Namespace: "d8-system",
			Labels: map[string]string{
				"heritage":                     "deckhouse",
				"app.kubernetes.io/managed-by": "Helm",
			},
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "deckhouse",
				"meta.helm.sh/release-namespace": "d8-system",
			},
		},
		Spec: appsv1.DeploymentSpec{
			RevisionHistoryLimit: pointer.Int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "deckhouse",
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}

	deckhousePodTemplate := apiv1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: deckhouseDeployment.Spec.Selector.MatchLabels,
		},
		Spec: apiv1.PodSpec{
			HostNetwork:        true,
			DNSPolicy:          apiv1.DNSDefault,
			ServiceAccountName: "deckhouse",
			Tolerations: []apiv1.Toleration{
				{Operator: apiv1.TolerationOpExists},
			},
			Volumes: []apiv1.Volume{
				{
					Name: "tmp",
					VolumeSource: apiv1.VolumeSource{
						EmptyDir: &apiv1.EmptyDirVolumeSource{Medium: apiv1.StorageMediumMemory},
					},
				},
				{
					Name: "kube",
					VolumeSource: apiv1.VolumeSource{
						EmptyDir: &apiv1.EmptyDirVolumeSource{Medium: apiv1.StorageMediumMemory},
					},
				},
			},
			Affinity: &apiv1.Affinity{
				NodeAffinity: &apiv1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
						NodeSelectorTerms: []apiv1.NodeSelectorTerm{
							{
								MatchExpressions: []apiv1.NodeSelectorRequirement{
									{
										Key:      ConvergeLabel,
										Operator: apiv1.NodeSelectorOpDoesNotExist,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	deckhouseContainer := apiv1.Container{
		Name:            "deckhouse",
		Image:           params.Registry,
		ImagePullPolicy: apiv1.PullAlways,
		Command: []string{
			"/deckhouse/deckhouse",
		},
		WorkingDir: "/deckhouse",
		ReadinessProbe: &apiv1.Probe{
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			FailureThreshold:    120,
			Handler: apiv1.Handler{
				HTTPGet: &apiv1.HTTPGetAction{
					Path: "/ready",
					Port: intstr.FromInt(9650),
				},
			},
		},
		Ports: []apiv1.ContainerPort{
			{Name: "self", ContainerPort: 9650},
			{Name: "custom", ContainerPort: 9651},
		},
	}

	deckhouseContainerEnv := []apiv1.EnvVar{
		{
			Name: "DECKHOUSE_POD",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"},
			},
		},
		{
			Name:  "HELM_HOST",
			Value: "127.0.0.1:44434",
		},
		{
			Name:  "HELM3LIB",
			Value: "yes",
		},
		{
			Name:  "HELM_HISTORY_MAX",
			Value: "3",
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
				FieldRef: &apiv1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
			},
		},
		{
			Name: "ADDON_OPERATOR_LISTEN_ADDRESS",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "status.podIP"},
			},
		},
		{
			Name:  "LOG_LEVEL",
			Value: params.LogLevel,
		},
		{
			Name:  "DECKHOUSE_BUNDLE",
			Value: params.Bundle,
		},
	}

	// Deployment composition
	deckhouseContainer.Env = deckhouseContainerEnv
	deckhousePodTemplate.Spec.Containers = []apiv1.Container{deckhouseContainer}
	deckhouseDeployment.Spec.Template = deckhousePodTemplate

	return ParameterizeDeckhouseDeployment(deckhouseDeployment, params)
}

func DeckhouseNamespace(name string) *apiv1.Namespace {
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

func DeckhouseServiceAccount() *apiv1.ServiceAccount {
	return &apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
			Labels: map[string]string{
				"heritage":                     "deckhouse",
				"app.kubernetes.io/managed-by": "Helm",
			},
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "deckhouse",
				"meta.helm.sh/release-namespace": "d8-system",
			},
		},
	}
}

func DeckhouseAdminClusterRole() *rbacv1.ClusterRole {
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

func DeckhouseAdminClusterRoleBinding() *rbacv1.ClusterRoleBinding {
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

func DeckhouseRegistrySecret(registry config.RegistryData) *apiv1.Secret {
	data, _ := base64.StdEncoding.DecodeString(registry.DockerCfg)
	ret := &apiv1.Secret{
		Type: apiv1.SecretTypeDockerConfigJson,
		ObjectMeta: metav1.ObjectMeta{
			Name: deckhouseRegistrySecretName,
			Labels: map[string]string{
				"heritage":                     "deckhouse",
				"app.kubernetes.io/managed-by": "Helm",
				"app":                          "registry",
			},
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "deckhouse",
				"meta.helm.sh/release-namespace": "d8-system",
			},
		},
		Data: map[string][]byte{
			apiv1.DockerConfigJsonKey: data,
			"address":                 []byte(registry.Address),
			"scheme":                  []byte(registry.Scheme),
		},
	}

	if registry.Path != "" {
		ret.Data["path"] = []byte(registry.Path)
	}

	if registry.CA != "" {
		ret.Data["ca"] = []byte(registry.CA)
	}

	return ret
}

func DeckhouseConfigMap(deckhouseConfig map[string]interface{}) *apiv1.ConfigMap {
	configMap := apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
	}

	var allErrs *multierror.Error

	configMapData := make(map[string]string, len(deckhouseConfig))
	for setting, data := range deckhouseConfig {
		if strings.HasSuffix(setting, "Enabled") {
			boolData, ok := data.(bool)
			if !ok {
				allErrs = multierror.Append(allErrs,
					fmt.Errorf("deckhouse config map validation: %q must be bool, option will be skipped", setting),
				)
			} else {
				configMapData[setting] = strconv.FormatBool(boolData)
			}
			continue
		}

		convertedData, err := yaml.Marshal(data)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("preparing deckhouse config map error (probably validation bug): %v", err))
			continue
		}
		configMapData[setting] = string(convertedData)
	}

	err := allErrs.ErrorOrNil()
	if err != nil {
		log.ErrorLn(err)
	}

	configMap.Data = configMapData
	return &configMap
}

func generateSecret(name, namespace string, data map[string][]byte, labels map[string]string) *apiv1.Secret {
	preparedLabels := map[string]string{"heritage": "deckhouse", "name": name}
	for key, value := range labels {
		preparedLabels[key] = value
	}
	return &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    preparedLabels,
		},
		Data: data,
	}
}

const TerraformClusterStateName = "d8-cluster-terraform-state"

func SecretWithTerraformState(data []byte) *apiv1.Secret {
	return generateSecret(
		TerraformClusterStateName,
		"d8-system",
		map[string][]byte{
			"cluster-tf-state.json": data,
		},
		nil,
	)
}

func PatchWithTerraformState(stateData []byte) interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"cluster-tf-state.json": stateData,
		},
	}
}

func SecretWithClusterConfig(data []byte) *apiv1.Secret {
	return generateSecret(
		"d8-cluster-configuration",
		"kube-system",
		map[string][]byte{"cluster-configuration.yaml": data},
		nil,
	)
}

func SecretWithProviderClusterConfig(configData, discoveryData []byte) *apiv1.Secret {
	data := make(map[string][]byte)
	if configData != nil {
		data["cloud-provider-cluster-configuration.yaml"] = configData
	}

	if discoveryData != nil {
		data["cloud-provider-discovery-data.json"] = discoveryData
	}

	return generateSecret("d8-provider-cluster-configuration", "kube-system", data, nil)
}

func SecretWithStaticClusterConfig(configData []byte) *apiv1.Secret {
	data := make(map[string][]byte)
	if configData != nil {
		data["static-cluster-configuration.yaml"] = configData
	}

	return generateSecret("d8-static-cluster-configuration", "kube-system", data, nil)
}

func SecretNameForNodeTerraformState(nodeName string) string {
	return "d8-node-terraform-state-" + nodeName
}

func SecretWithNodeTerraformState(nodeName, nodeGroup string, data, settings []byte) *apiv1.Secret {
	body := map[string][]byte{"node-tf-state.json": data}
	if settings != nil {
		body["node-group-settings.json"] = settings
	}
	return generateSecret(
		SecretNameForNodeTerraformState(nodeName),
		"d8-system",
		body,
		map[string]string{
			"node.deckhouse.io/node-group":      nodeGroup,
			"node.deckhouse.io/node-name":       nodeName,
			"node.deckhouse.io/terraform-state": "",
		},
	)
}

func PatchWithNodeTerraformState(stateData []byte) interface{} {
	return map[string]interface{}{
		"data": map[string]interface{}{
			"node-tf-state.json": stateData,
		},
	}
}

func SecretMasterDevicePath(nodeName string, devicePath []byte) *apiv1.Secret {
	return generateSecret(
		"d8-masters-kubernetes-data-device-path",
		"d8-system",
		map[string][]byte{
			nodeName: devicePath,
		},
		map[string]string{},
	)
}

const (
	ClusterUUIDCmKey       = "cluster-uuid"
	ClusterUUIDCm          = "d8-cluster-uuid"
	ClusterUUIDCmNamespace = "kube-system"
)

func ClusterUUIDConfigMap(uuid string) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterUUIDCm,
			Namespace: ClusterUUIDCmNamespace,
		},
		Data: map[string]string{ClusterUUIDCmKey: uuid},
	}
}

func KubeDNSService(ipAddress string) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-dns",
			Namespace: metav1.NamespaceSystem,
			Labels: map[string]string{
				"k8s-app": "kube-dns",
			},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: ipAddress,
			Ports: []apiv1.ServicePort{
				{
					Name:       "dns",
					Port:       53,
					Protocol:   "UDP",
					TargetPort: intstr.FromInt(53),
				},
				{
					Name:       "dns-tcp",
					Port:       53,
					TargetPort: intstr.FromInt(53),
				},
			},
			Selector: map[string]string{
				"k8s-app": "kube-dns",
			},
		},
	}
}
