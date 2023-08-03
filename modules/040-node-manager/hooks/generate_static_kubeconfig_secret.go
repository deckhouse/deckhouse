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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"os"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/kubeconfig"
)

const (
	clusterAPINamespace          = "d8-cloud-instance-manager"
	clusterAPIServiceAccountName = "capi-controller-manager"
)

//var (
//	clusterAPIClusterGVR = schema.GroupVersionResource{
//		Group:    "cluster.x-k8s.io",
//		Version:  "v1beta1",
//		Resource: "clusters",
//	}
//)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_api_cluster",
			ApiVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Cluster",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{clusterAPINamespace},
				},
			},
			FilterFunc: applyClusterFilter,
		},
	},
}, dependency.WithExternalDependencies(generateStaticKubeconfigSecret))

func applyClusterFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

func generateStaticKubeconfigSecret(input *go_hook.HookInput, dc dependency.Container) error {
	if len(input.Snapshots["cluster_api_cluster"]) == 0 {
		return nil
	}

	cluster := input.Snapshots["cluster_api_cluster"][0].(*unstructured.Unstructured)

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return errors.Wrap(err, "failed to get k8s client")
	}

	//tokenAudience := fmt.Sprintf("https://kubernetes.default.svc.%s", input.Values.Get("global.clusterConfiguration.clusterDomain").String())
	//
	//tokenExpirationSeconds := int64((10 * time.Minute).Seconds())
	//
	//tokenReq := &authenticationv1.TokenRequest{
	//	Spec: authenticationv1.TokenRequestSpec{
	//		Audiences:         []string{tokenAudience},
	//		ExpirationSeconds: &tokenExpirationSeconds,
	//	},
	//}
	//
	//tokenReq, err = k8sClient.CoreV1().ServiceAccounts(clusterAPINamespace).CreateToken(context.TODO(), clusterAPIServiceAccountName, tokenReq, metav1.CreateOptions{})
	//if err != nil {
	//	return errors.Wrapf(err, "failed to create an audience scoped token for '%s' ServiceAccount", clusterAPIServiceAccountName)
	//}

	//cluster, err := k8sClient.Dynamic().Resource(clusterAPIClusterGVR).Namespace(clusterAPINamespace).Get(context.TODO(), clusterAPIStaticClusterName, metav1.GetOptions{})
	//if err != nil {
	//	return errors.Wrap(err, "failed to get cluster")
	//}

	secretForServiceAccountToken := kubeconfig.GenerateSecretForServiceAccountToken(cluster, clusterAPIServiceAccountName)

	//secretUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secretForServiceAccountToken)
	//if err != nil {
	//	return errors.Wrap(err, "failed to convert secret to unstructured")
	//}
	//
	//input.PatchCollector.Create(secretUnstructured, object_patch.UpdateIfExists())

	_, err = k8sClient.CoreV1().Secrets(secretForServiceAccountToken.Namespace).Create(context.TODO(), secretForServiceAccountToken, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "failed to create secret")
		}
	}

	secretForServiceAccountToken, err = k8sClient.CoreV1().Secrets(secretForServiceAccountToken.Namespace).Get(context.TODO(), secretForServiceAccountToken.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get secret")
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
