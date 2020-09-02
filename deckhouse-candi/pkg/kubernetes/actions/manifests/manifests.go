package manifests

import (
	"encoding/base64"
	"flant/deckhouse-candi/pkg/log"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	deckhouseRegistrySecretName = "deckhouse-registry"
	deckhouseRegistryVolumeName = "registrysecret"
)

//nolint:funlen
func DeckhouseDeployment(registry, logLevel, bundle string, isSecureRegistry bool) *appsv1.Deployment {
	var deckhouseDeployment = `
kind: Deployment
apiVersion: apps/v1
metadata:
  name: deckhouse
  namespace: d8-system
  labels:
    heritage: deckhouse
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
        - name: KUBERNETES_DEPLOYED
          value: PLACEHOLDER
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
        resources:
          requests:
            cpu: 50m
            memory: 512Mi
        workingDir: /deckhouse
      hostNetwork: true
      dnsPolicy: Default
      serviceAccountName: deckhouse
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - operator: Exists
`

	var deployment appsv1.Deployment
	_ = yaml.Unmarshal([]byte(deckhouseDeployment), &deployment)

	deployment.Spec.Template.Spec.Containers[0].Image = registry

	for i, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "LOG_LEVEL":
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = logLevel
		case "DECKHOUSE_BUNDLE":
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = bundle
		case "KUBERNETES_DEPLOYED":
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = time.Now().Format(time.RFC3339)
		}
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
				"heritage": "deckhouse",
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

func DeckhouseRegistrySecret(dockerCfg string) *apiv1.Secret {
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

func DeckhouseConfigMap(deckhouseConfig map[string]interface{}) *apiv1.ConfigMap {
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
				log.ErrorF("deckhouse config map: %q must be boo\n", setting)
			}
			configMapData[setting] = strconv.FormatBool(boolData)

			continue
		}
		convertedData, err := yaml.Marshal(data)
		if err != nil {
			log.ErrorF("preparing deckhouse config map error (probably validation bug): %v", err)
			continue
		}
		configMapData[setting] = string(convertedData)
	}
	configMap.Data = configMapData
	return &configMap
}

func generateSecret(name, namespace string, data map[string][]byte, labels map[string]string) *apiv1.Secret {
	preparedLabels := map[string]string{"heritage": "deckhouse"}
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

func SecretWithTerraformState(data []byte) *apiv1.Secret {
	return generateSecret(
		"d8-cluster-terraform-state",
		"d8-system",
		map[string][]byte{
			"cluster-tf-state.json": data,
		},
		nil,
	)
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
	return generateSecret(
		"d8-provider-cluster-configuration",
		"kube-system",
		map[string][]byte{
			"cloud-provider-cluster-configuration.yaml": configData,
			"cloud-provider-discovery-data.json":        discoveryData,
		},
		nil,
	)
}

func SecretWithNodeTerraformState(nodeName, nodeGroup string, data, settings []byte) *apiv1.Secret {
	body := map[string][]byte{"node-tf-state.json": data}
	if settings != nil {
		body["node-group-settings.json"] = settings
	}
	return generateSecret(
		"d8-node-terraform-state-"+nodeName,
		"d8-system",
		body,
		map[string]string{
			"node.deckhouse.io/node-group":      nodeGroup,
			"node.deckhouse.io/node-name":       nodeName,
			"node.deckhouse.io/terraform-state": "",
		},
	)
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
