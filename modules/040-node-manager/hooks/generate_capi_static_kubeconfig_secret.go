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
	"os"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
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

var (
	clusterAPIClusterGVR = schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "clusters",
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "capi_static_kubeconfig_secret",
			Crontab: "0 1 * * *",
		},
	},
}, dependency.WithExternalDependencies(generateStaticKubeconfigSecret))

func generateStaticKubeconfigSecret(input *go_hook.HookInput, dc dependency.Container) error {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return errors.Wrap(err, "failed to get k8s client")
	}

	cluster, err := k8sClient.Dynamic().Resource(clusterAPIClusterGVR).Namespace(clusterAPINamespace).Get(context.TODO(), clusterAPIStaticClusterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return errors.Wrap(err, "failed to get cluster")
	}

	secretForServiceAccountToken, err := getSecretForServiceAccountToken(k8sClient, cluster)
	if err != nil {
		return errors.Wrap(err, "failed to get secret for service account token")
	}

	serviceAccountToken, ok := secretForServiceAccountToken.Data["token"]
	if !ok {
		return errors.New("service account token not found")
	}

	caCert, err := os.ReadFile(restConfig.TLSClientConfig.CAFile)
	if err != nil {
		return errors.Wrap(err, "failed to read CA file")
	}

	config, err := kubeconfig.New(cluster.GetName(), restConfig.Host, caCert, string(serviceAccountToken))
	if err != nil {
		return errors.Wrap(err, "failed to generate a kubeconfig")
	}

	configYAML, err := clientcmd.Write(*config)
	if err != nil {
		return errors.Wrap(err, "failed to serialize kubeconfig to yaml")
	}

	secret := kubeconfig.GenerateSecret(cluster, configYAML)

	secretUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return errors.Wrap(err, "failed to convert secret to unstructured")
	}

	input.PatchCollector.Create(secretUnstructured, object_patch.UpdateIfExists())

	return nil
}

func getSecretForServiceAccountToken(k8sClient k8s.Client, cluster *unstructured.Unstructured) (*v1.Secret, error) {
	secret, err := k8sClient.CoreV1().Secrets(cluster.GetNamespace()).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig-token", cluster.GetName()), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			secret = kubeconfig.GenerateSecretForServiceAccountToken(cluster, clusterAPIServiceAccountName)

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
