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
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	deckhouseRegistrySecretName = "deckhouse-registry"
	deckhouseRegistryVolumeName = "registrysecret"

	deployTimeEnvVarName   = "KUBERNETES_DEPLOYED"
	deployTimeEnvVarFormat = time.RFC3339
)

type DeckhouseDeploymentParams struct {
	Bundle           string
	Registry         string
	LogLevel         string
	DeployTime       time.Time
	IsSecureRegistry bool
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

func ParametrizeDeckhouseDeployment(deployment *appsv1.Deployment, params DeckhouseDeploymentParams) *appsv1.Deployment {
	deployment.Spec.Template.Spec.Containers[0].Image = params.Registry

	deployTime := params.DeployTime
	if deployTime.IsZero() {
		deployTime = time.Now()
	}

	for i, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "LOG_LEVEL":
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = params.LogLevel
		case "DECKHOUSE_BUNDLE":
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = params.Bundle
		case deployTimeEnvVarName:
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = deployTime.Format(deployTimeEnvVarFormat)
		}
	}

	if params.IsSecureRegistry {
		deployment.Spec.Template.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{
			{Name: deckhouseRegistrySecretName},
		}

		// set volume

		volumeToSet := apiv1.Volume{
			Name: deckhouseRegistryVolumeName,
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{SecretName: deckhouseRegistrySecretName},
			},
		}

		volumeIndx := -1

		for i := range deployment.Spec.Template.Spec.Volumes {
			if deployment.Spec.Template.Spec.Volumes[i].Name == deckhouseRegistryVolumeName {
				volumeIndx = i
				break
			}
		}

		if volumeIndx < 0 {
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volumeToSet)
		} else {
			deployment.Spec.Template.Spec.Volumes[volumeIndx] = volumeToSet
		}

		// set volume mount

		volumeMountToSet := apiv1.VolumeMount{
			Name:      deckhouseRegistryVolumeName,
			MountPath: "/etc/registrysecret",
			ReadOnly:  true,
		}

		volumeMountIndx := -1

		for i := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
			if deployment.Spec.Template.Spec.Containers[0].VolumeMounts[i].Name == deckhouseRegistryVolumeName {
				volumeMountIndx = i
				break
			}
		}

		if volumeMountIndx < 0 {
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMountToSet)
		} else {
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts[volumeMountIndx] = volumeMountToSet
		}
	}

	return deployment
}

//nolint:funlen
func DeckhouseDeployment(params DeckhouseDeploymentParams) *appsv1.Deployment {
	deckhouseDeployment := `
kind: Deployment
apiVersion: apps/v1
metadata:
  name: deckhouse
  namespace: d8-system
  labels:
    heritage: deckhouse
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: deckhouse
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: deckhouse
    spec:
      containers:
      - name: deckhouse
        image: PLACEHOLDER
        command:
        - /deckhouse/deckhouse
        imagePullPolicy: Always
        env:
# KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT is needed on bootstrap phase to deckhouse work without kube-proxy
        - name: KUBERNETES_SERVICE_HOST
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.hostIP
        - name: KUBERNETES_SERVICE_PORT
          value: "6443"
        - name: LOG_LEVEL
          value: PLACEHOLDER
        - name: DECKHOUSE_BUNDLE
          value: PLACEHOLDER
        - name: DECKHOUSE_POD
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: HELM_HOST
          value: "127.0.0.1:44434"
        - name: ADDON_OPERATOR_CONFIG_MAP
          value: deckhouse
        - name: ADDON_OPERATOR_PROMETHEUS_METRICS_PREFIX
          value: deckhouse_
        - name: ADDON_OPERATOR_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: ADDON_OPERATOR_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: HELM3LIB
          value: "yes"
        - name: KUBERNETES_DEPLOYED
          value: PLACEHOLDER
        volumeMounts:
        - mountPath: /tmp
          name: tmp
        - mountPath: /.kube
          name: kube
        ports:
        - containerPort: 9650
          name: self
        - containerPort: 9651
          name: custom
        readinessProbe:
          httpGet:
            path: /ready
            port: 9650
          initialDelaySeconds: 5
          # fail after 10 minutes
          periodSeconds: 5
          failureThreshold: 120
        workingDir: /deckhouse
      hostNetwork: true
      dnsPolicy: Default
      serviceAccountName: deckhouse
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - operator: Exists
      volumes:
      - emptyDir:
          medium: Memory
        name: tmp
      - emptyDir:
          medium: Memory
        name: kube
`

	var deployment appsv1.Deployment
	err := yaml.Unmarshal([]byte(deckhouseDeployment), &deployment)
	if err != nil {
		panic(err)
	}

	return ParametrizeDeckhouseDeployment(&deployment, params)
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

func ClusterUUIDConfigMap(uuid string) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cluster-uuid",
			Namespace: "kube-system",
		},
		Data: map[string]string{"cluster-uuid": uuid},
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
