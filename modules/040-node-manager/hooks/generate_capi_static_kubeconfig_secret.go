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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/kubeconfig"
)

const (
	clusterAPINamespace          = "d8-cloud-instance-manager"
	clusterAPIServiceAccountName = "capi-controller-manager"
	clusterAPIStaticClusterName  = "static"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/capi",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: staticInstancesNodeGroupFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "capi_static_kubeconfig_secret",
			Crontab: "0 1 * * *",
		},
	},
}, dependency.WithExternalDependencies(generateStaticKubeconfigSecret))

func generateStaticKubeconfigSecret(input *go_hook.HookInput, dc dependency.Container) error {
	var hasStaticInstancesField bool

	nodeGroupSnapshots := input.Snapshots["node_group"]
	for _, nodeGroupSnapshot := range nodeGroupSnapshots {
		hasStaticInstancesField = nodeGroupSnapshot.(nodeGroupWithStaticInstances).HasStaticInstances
		if hasStaticInstancesField {
			break // we need at least one NodeGroup with staticInstances field
		}
	}

	if !hasStaticInstancesField {
		return nil
	}

	restConfig, err := dc.GetClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return errors.Wrap(err, "failed to get k8s client")
	}

	err = createCAPIServiceAccount(k8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to create Cluster API service account")
	}

	secretForServiceAccountToken, err := getSecretForServiceAccountToken(clusterAPIStaticClusterName, clusterAPINamespace, k8sClient)
	if err != nil {
		return errors.Wrap(err, "failed to get secret for service account token")
	}

	serviceAccountToken, ok := secretForServiceAccountToken.Data["token"]
	if !ok {
		return errors.New("service account token not found")
	}

	config, err := kubeconfig.New(clusterAPIStaticClusterName, restConfig.Host, restConfig.CAData, string(serviceAccountToken))
	if err != nil {
		return errors.Wrap(err, "failed to generate a kubeconfig")
	}

	configYAML, err := clientcmd.Write(*config)
	if err != nil {
		return errors.Wrap(err, "failed to serialize kubeconfig to yaml")
	}

	secret := kubeconfig.GenerateSecret(clusterAPIStaticClusterName, clusterAPINamespace, configYAML)

	secretUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return errors.Wrap(err, "failed to convert secret to unstructured")
	}

	input.PatchCollector.Create(secretUnstructured, object_patch.UpdateIfExists())

	return nil
}

func createCAPIServiceAccount(k8sClient k8s.Client) error {
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterAPIServiceAccountName,
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

func getSecretForServiceAccountToken(clusterName string, namespace string, k8sClient k8s.Client) (*v1.Secret, error) {
	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig-token", clusterName), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			secret = kubeconfig.GenerateSecretForServiceAccountToken(clusterName, namespace, clusterAPIServiceAccountName)

			_, err = k8sClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return nil, errors.Wrap(err, "failed to create secret")
				}
			}

			secret, err = k8sClient.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrap(err, "failed to get secret after creation")
			}

			return secret, nil
		}

		return nil, errors.Wrap(err, "failed to get secret")
	}

	return secret, nil
}
