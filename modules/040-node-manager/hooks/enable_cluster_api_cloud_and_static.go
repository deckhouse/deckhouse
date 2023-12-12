/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"context"
	"os"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/kubeconfig"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	clusterAPINamespace                = "d8-cloud-instance-manager"
	clusterAPIStaticServiceAccountName = "capi-controller-manager"
	clusterAPIStaticClusterName        = "static"
	clusterAPICloudServiceAccountName  = "capi-cloud-cluster-controller-manager"
)

type hookParam struct {
	serviceAccount string
	cluster        string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/node-manager",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "capi_static_kubeconfig_secret",
			Crontab: "0 1 * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: staticInstancesNodeGroupFilter,
		},
		{
			Name:       "config_map",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{clusterAPINamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"capi-controller-manager"},
			},
			FilterFunc: capsConfigMapFilter,
		},
	},
}, dependency.WithExternalDependencies(handleClusterAPIDeploymentRequired))

func staticInstancesNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return ng.Spec.StaticInstances != nil, nil
}

func capsConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var configMap corev1.ConfigMap

	err := sdk.FromUnstructured(obj, &configMap)
	if err != nil {
		return nil, err
	}

	enable, ok := configMap.Data["enable"]
	if !ok {
		return nil, nil
	}

	return enable == "true", nil
}

func handleClusterAPIDeploymentRequired(input *go_hook.HookInput, dc dependency.Container) error {
	var hasStaticInstancesField bool

	nodeGroupSnapshots := input.Snapshots["node_group"]
	for _, nodeGroupSnapshot := range nodeGroupSnapshots {
		hasStaticInstancesField = nodeGroupSnapshot.(bool)
		if hasStaticInstancesField {
			break // we need at least one NodeGroup with staticInstances field
		}
	}

	capiClusterName := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
	hasCapiProvider := capiClusterName != ""

	var capiEnabled bool
	var capsEnabled bool

	configMapSnapshots := input.Snapshots["config_map"]
	if len(configMapSnapshots) > 0 {
		capiEnabled = hasCapiProvider || configMapSnapshots[0].(bool)
		capsEnabled = configMapSnapshots[0].(bool)
	} else {
		capiEnabled = hasCapiProvider || hasStaticInstancesField
	}

	if capiEnabled {
		input.Values.Set("nodeManager.internal.capiControllerManagerEnabled", true)
		if _, ok := os.LookupEnv("TEST_SKIP_GENERATE_KUBECONFIG"); !ok {
			err := generateStaticKubeconfigSecret(input, dc, hookParam{
				serviceAccount: clusterAPICloudServiceAccountName,
				cluster:        capiClusterName,
			})
			if err != nil {
				return err
			}
		}
	} else {
		input.Values.Remove("nodeManager.internal.capiControllerManagerEnabled")
	}

	if capsEnabled || hasStaticInstancesField {
		if _, ok := os.LookupEnv("TEST_SKIP_GENERATE_KUBECONFIG"); !ok {
			err := generateStaticKubeconfigSecret(input, dc, hookParam{
				serviceAccount: clusterAPIStaticServiceAccountName,
				cluster:        clusterAPIStaticClusterName,
			})
			if err != nil {
				return err
			}
		}

		input.Values.Set("nodeManager.internal.capsControllerManagerEnabled", true)
	} else {
		input.Values.Remove("nodeManager.internal.capsControllerManagerEnabled")
	}

	return nil
}

func generateStaticKubeconfigSecret(input *go_hook.HookInput, dc dependency.Container, params hookParam) error {
	restConfig, err := dc.GetClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return errors.Wrap(err, "failed to get k8s client")
	}

	err = createCAPIServiceAccount(k8sClient, params.serviceAccount)
	if err != nil {
		return errors.Wrap(err, "failed to create Cluster API service account")
	}

	certExirationSeconds := int32((180 * 24 * time.Hour).Seconds())

	cert, err := tls_certificate.IssueCertificate(input, dc, tls_certificate.OrderCertificateRequest{
		CommonName: "capi-controller-manager",
		Groups: []string{
			"d8:node-manager:capi-controller-manager:manager-role",
		},
		Usages: []certificatesv1.KeyUsage{
			certificatesv1.UsageClientAuth,
		},
		ExpirationSeconds: &certExirationSeconds,
	})
	if err != nil {
		return errors.Wrap(err, "failed to issue certificate")
	}

	config, err := kubeconfig.New(params.cluster, restConfig.Host, restConfig.CAData, []byte(cert.Key), []byte(cert.Certificate))
	if err != nil {
		return errors.Wrap(err, "failed to generate a kubeconfig")
	}

	configYAML, err := clientcmd.Write(*config)
	if err != nil {
		return errors.Wrap(err, "failed to serialize kubeconfig to yaml")
	}

	secret := kubeconfig.GenerateSecret(params.cluster, clusterAPINamespace, configYAML)

	secretUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return errors.Wrap(err, "failed to convert secret to unstructured")
	}

	input.PatchCollector.Create(secretUnstructured, object_patch.UpdateIfExists())

	return nil
}

func createCAPIServiceAccount(k8sClient k8s.Client, saName string) error {
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: clusterAPINamespace,
		},
	}

	_, err := k8sClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "failed to create service account")
		}
	}

	return nil
}
