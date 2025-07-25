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
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var imagesDigestsJSON = "/deckhouse/candi/images_digests.json"

const (
	deckhouseRegistrySecretName = "deckhouse-registry"

	deployTimeEnvVarName            = "KUBERNETES_DEPLOYED"
	deployServiceHostEnvVarName     = "KUBERNETES_SERVICE_HOST"
	deployServicePortEnvVarName     = "KUBERNETES_SERVICE_PORT"
	deployTimeEnvVarFormat          = time.RFC3339
	pathSeparator                   = ":"
	NodeInfrastructureStateLabelKey = "node.deckhouse.io/terraform-state"
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

type imagesDigests map[string]map[string]interface{}

func loadImagesDigests(filename string) (imagesDigests, error) {
	if val, ok := os.LookupEnv("DHCTL_TEST"); ok && val == "yes" {
		return map[string]map[string]interface{}{
			"common": {
				"init": "sha256:4c5064aa2864e7650e4f2dd5548a4a6a4aaa065b4f8779f01023f73132cde882",
			},
		}, nil
	}

	var imagesDigestsDict imagesDigests

	imagesDigestsJSONFile, err := os.ReadFile(filename)
	if err != nil {
		return imagesDigestsDict, fmt.Errorf("%s file load: %v", filename, err)
	}

	err = yaml.Unmarshal(imagesDigestsJSONFile, &imagesDigestsDict)
	if err != nil {
		return imagesDigestsDict, fmt.Errorf("%s file unmarshal: %v", filename, err)
	}

	return imagesDigestsDict, nil
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
	initContainerImage := params.Registry
	imagesDigestsDict, err := loadImagesDigests(imagesDigestsJSON)
	if err != nil {
		log.ErrorLn(err)
	} else {
		imageSplitIndex := strings.LastIndex(params.Registry, ":")
		initContainerImage = fmt.Sprintf("%s@%s", params.Registry[:imageSplitIndex], imagesDigestsDict["common"]["init"].(string))
	}

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
			RevisionHistoryLimit: ptr.To(int32(0)),
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

	hostPathDirectory := apiv1.HostPathDirectoryOrCreate

	deckhousePodTemplate := apiv1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: deckhouseDeployment.Spec.Selector.MatchLabels,
			Annotations: map[string]string{
				"kubectl.kubernetes.io/default-container": "deckhouse",
			},
		},
		Spec: apiv1.PodSpec{
			HostNetwork:                  true,
			DNSPolicy:                    apiv1.DNSDefault,
			ServiceAccountName:           "deckhouse",
			AutomountServiceAccountToken: ptr.To(true),
			SecurityContext: &apiv1.PodSecurityContext{
				RunAsUser:    ptr.To(int64(0)),
				RunAsNonRoot: ptr.To(false),
			},
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
				{
					Name: "downloaded",
					VolumeSource: apiv1.VolumeSource{
						HostPath: &apiv1.HostPathVolumeSource{
							Path: "/var/lib/deckhouse/downloaded",
							Type: &hostPathDirectory,
						},
					},
				},
				{
					Name: "deckhouse",
					VolumeSource: apiv1.VolumeSource{
						HostPath: &apiv1.HostPathVolumeSource{
							Path: "/var/lib/deckhouse",
							Type: &hostPathDirectory,
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
			"/usr/bin/deckhouse-controller",
			"start",
		},
		WorkingDir: "/deckhouse",
		ReadinessProbe: &apiv1.Probe{
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			FailureThreshold:    120,
			ProbeHandler: apiv1.ProbeHandler{
				HTTPGet: &apiv1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.FromInt(4222),
				},
			},
		},
		Ports: []apiv1.ContainerPort{
			{Name: "self", ContainerPort: 4222},
			{Name: "custom", ContainerPort: 4223},
		},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "tmp",
				ReadOnly:  false,
				MountPath: "/tmp",
			},
			{
				Name:      "kube",
				ReadOnly:  false,
				MountPath: "/.kube",
			},
			{
				Name:      "downloaded",
				ReadOnly:  false,
				MountPath: "/deckhouse/downloaded",
			},
		},
	}

	deckhouseInitContainer := apiv1.Container{
		Name:            "init-downloaded-modules",
		Image:           initContainerImage,
		ImagePullPolicy: apiv1.PullAlways,
		Command: []string{
			"sh", "-c", `if [ -d "/deckhouse/external-modules" ] && [ -n "$(ls -A "/deckhouse/external-modules")" ]; then cp -r /deckhouse/external-modules/* /deckhouse/downloaded/ && rm -rf /deckhouse/external-modules; fi && mkdir -p /deckhouse/downloaded/modules && chown -hR 64535 /deckhouse/downloaded /deckhouse/downloaded/modules && chmod 0700 /deckhouse/downloaded /deckhouse/downloaded/modules`,
		},
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      "deckhouse",
				ReadOnly:  false,
				MountPath: "/deckhouse",
			},
		},
	}

	modulesDirs := []string{
		"/deckhouse/modules",
		"/deckhouse/downloaded",
	}

	deckhouseContainerEnv := []apiv1.EnvVar{
		{
			Name: "DECKHOUSE_POD",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"},
			},
		},
		{
			Name: "DECKHOUSE_NODE_NAME",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
			},
		},
		{
			Name:  "HELM_HOST",
			Value: "127.0.0.1:44434",
		},
		{
			Name:  "ADDON_OPERATOR_LISTEN_PORT",
			Value: "4222",
		},
		{
			Name:  "ADDON_OPERATOR_ADMISSION_SERVER_LISTEN_PORT",
			Value: "4223",
		},
		{
			Name:  "ADDON_OPERATOR_CRD_EXTRA_LABELS",
			Value: "heritage=deckhouse",
		},
		{
			Name:  "HELM_HISTORY_MAX",
			Value: "3",
		},
		{
			Name:  "GOGC",
			Value: "50",
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
			Name:  "LOG_TYPE",
			Value: "json",
		},
		{
			Name:  "DECKHOUSE_BUNDLE",
			Value: params.Bundle,
		},
		{
			Name:  "DEBUG_HTTP_SERVER_ADDR",
			Value: "127.0.0.1:9652",
		},
		{
			Name:  "ADDON_OPERATOR_APPLIED_MODULE_EXTENDERS",
			Value: "Static,DynamicallyEnabled,KubeConfig,DeckhouseVersion,KubernetesVersion,Bootstrapped,ScriptEnabled,ModuleDependency",
		},
		{
			Name:  "DOWNLOADED_MODULES_DIR",
			Value: modulesDirs[1],
		},
		{
			Name:  "EXTERNAL_MODULES_DIR",
			Value: modulesDirs[1],
		},
		{
			Name:  "MODULES_DIR",
			Value: strings.Join(modulesDirs, pathSeparator),
		},
	}

	// Deployment composition
	deckhouseContainer.Env = deckhouseContainerEnv
	deckhousePodTemplate.Spec.Containers = []apiv1.Container{deckhouseContainer}
	deckhousePodTemplate.Spec.InitContainers = []apiv1.Container{deckhouseInitContainer}
	deckhouseDeployment.Spec.Template = deckhousePodTemplate

	return ParameterizeDeckhouseDeployment(deckhouseDeployment, params)
}

func DeckhouseNamespace(name string) *apiv1.Namespace {
	return &apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"extended-monitoring.deckhouse.io/enabled": "",
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
		AutomountServiceAccountToken: ptr.To(false),
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
			"imagesRegistry":          []byte(registry.Address),
		},
	}

	if registry.Path != "" {
		ret.Data["path"] = []byte(registry.Path)
		ret.Data["imagesRegistry"] = []byte(registry.Address + registry.Path)
	}

	if registry.CA != "" {
		ret.Data["ca"] = []byte(registry.CA)
	}

	return ret
}

func generateSecret(name, namespace string, data map[string][]byte, labels map[string]string) *apiv1.Secret {
	preparedLabels := make(map[string]string)
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

const InfrastructureClusterStateName = "d8-cluster-terraform-state"

func SecretWithInfrastructureState(data []byte) *apiv1.Secret {
	return generateSecret(
		InfrastructureClusterStateName,
		"d8-system",
		map[string][]byte{
			"cluster-tf-state.json": data,
		},
		map[string]string{
			"heritage": "deckhouse",
		},
	)
}

func PatchWithInfrastructureState(stateData []byte) interface{} {
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
		map[string]string{"name": "d8-cluster-configuration"},
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

	return generateSecret(
		"d8-provider-cluster-configuration",
		"kube-system",
		data,
		nil,
	)
}

func SecretWithStaticClusterConfig(configData []byte) *apiv1.Secret {
	data := make(map[string][]byte)
	if configData != nil {
		data["static-cluster-configuration.yaml"] = configData
	}
	return generateSecret(
		"d8-static-cluster-configuration",
		"kube-system",
		data,
		nil,
	)
}

func SecretNameForNodeInfrastructureState(nodeName string) string {
	return "d8-node-terraform-state-" + nodeName
}

func SecretWithNodeInfrastructureState(nodeName, nodeGroup string, data, settings []byte) *apiv1.Secret {
	body := map[string][]byte{"node-tf-state.json": data}
	if settings != nil {
		body["node-group-settings.json"] = settings
	}
	return generateSecret(
		SecretNameForNodeInfrastructureState(nodeName),
		"d8-system",
		body,
		map[string]string{
			"node.deckhouse.io/node-group":  nodeGroup,
			"node.deckhouse.io/node-name":   nodeName,
			NodeInfrastructureStateLabelKey: "",
			"heritage":                      "deckhouse",
		},
	)
}

func PatchWithNodeInfrastructureState(stateData []byte) interface{} {
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
		map[string]string{
			"heritage": "deckhouse",
		},
	)
}

func SecretConvergeState(state []byte) *apiv1.Secret {
	return generateSecret(
		"d8-dhctl-converge-state",
		"d8-system",
		map[string][]byte{
			"state.json": state,
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

const (
	CommanderUUIDCmKey       = "commander-uuid"
	CommanderUUIDCm          = "d8-commander-uuid"
	CommanderUUIDCmNamespace = "kube-system"
)

func CommanderUUIDConfigMap(uuid string) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CommanderUUIDCm,
			Namespace: CommanderUUIDCmNamespace,
		},
		Data: map[string]string{CommanderUUIDCmKey: uuid},
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

func InitGlobalVars(pwd string) {
	imagesDigestsJSON = pwd + "/deckhouse/candi/images_digests.json"
}
