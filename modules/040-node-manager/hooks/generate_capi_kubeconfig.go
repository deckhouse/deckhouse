/*
Copyright 2026 Flant JSC

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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/kubeconfig"
)

const (
	clusterAPINamespace          = "d8-cloud-instance-manager"
	clusterAPIStaticClusterName  = "static"
	clusterAPIServiceAccountName = "capi-controller-manager"

	capiKubeconfigSnapshot         = "capi_kubeconfig_secrets"
	capiKubeconfigClusterNameLabel = "cluster.x-k8s.io/cluster-name"

	capiKubeconfigExpiresAtAnnotation    = "node-manager.deckhouse.io/certificate-expires-at"
	capiKubeconfigEndpointHashAnnotation = "node-manager.deckhouse.io/endpoint-hash"

	// Same margin as certificateHandlerWithRequests in go_lib/hooks/tls_certificate/order_certificate.go.
	capiKubeconfigRenewMargin = 15 * 24 * time.Hour
)

type capiKubeconfigSecret struct {
	Name         string `json:"name"`
	ExpiresAt    string `json:"expiresAt"`
	EndpointHash string `json:"endpointHash"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 100},
	Queue:        "/modules/node-manager",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "capi_kubeconfig_secret",
			Crontab: "0 1 * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       capiKubeconfigSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{clusterAPINamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      capiKubeconfigClusterNameLabel,
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			ExecuteHookOnEvents:          go_hook.Bool(false),
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   filterCAPIKubeconfigSecret,
		},
	},
}, dependency.WithExternalDependencies(handleCreateCAPIStaticKubeconfig))

func filterCAPIKubeconfigSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	annotations := obj.GetAnnotations()

	expiresAt := annotations[capiKubeconfigExpiresAtAnnotation]
	if expiresAt == "" {
		return nil, nil
	}

	return capiKubeconfigSecret{
		Name:         obj.GetName(),
		ExpiresAt:    expiresAt,
		EndpointHash: annotations[capiKubeconfigEndpointHashAnnotation],
	}, nil
}

func handleCreateCAPIStaticKubeconfig(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	capiEnabledRaw := input.Values.Get("nodeManager.internal.capiControllerManagerEnabled")

	if capiEnabledRaw.Exists() && capiEnabledRaw.Bool() {
		capiClusterName := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
		if capiClusterName != "" {
			err := generateKubeconfigSecret(input, dc, hookParam{
				serviceAccount: clusterAPIServiceAccountName,
				cluster:        capiClusterName,
			})
			if err != nil {
				return err
			}
		}
	}

	capsEnabledRaw := input.Values.Get("nodeManager.internal.capsControllerManagerEnabled")

	if capsEnabledRaw.Exists() && capsEnabledRaw.Bool() {
		err := generateKubeconfigSecret(input, dc, hookParam{
			serviceAccount: clusterAPIServiceAccountName,
			cluster:        clusterAPIStaticClusterName,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func generateKubeconfigSecret(input *go_hook.HookInput, dc dependency.Container, params hookParam) error {
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

	endpointHash := apiserverEndpointHash(restConfig.Host, restConfig.CAData)

	reusable, err := kubeconfigSecretIsReusable(input, params.cluster, endpointHash)
	if err != nil {
		return err
	}

	if reusable {
		return nil
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

	issuedCert, err := helpers.ParseCertificatePEM([]byte(cert.Certificate))
	if err != nil {
		return fmt.Errorf("parse issued certificate: %w", err)
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
	secret.Annotations = map[string]string{
		capiKubeconfigExpiresAtAnnotation:    issuedCert.NotAfter.UTC().Format(time.RFC3339),
		capiKubeconfigEndpointHashAnnotation: endpointHash,
	}

	secretUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return errors.Wrap(err, "failed to convert secret to unstructured")
	}

	input.PatchCollector.CreateOrUpdate(secretUnstructured)

	return nil
}

// kubeconfigSecretIsReusable reports whether the kubeconfig secret of the cluster still holds
// a long enough living certificate issued for the current apiserver endpoint.
func kubeconfigSecretIsReusable(input *go_hook.HookInput, cluster, endpointHash string) (bool, error) {
	secrets, err := sdkobjectpatch.UnmarshalToStruct[capiKubeconfigSecret](input.Snapshots, capiKubeconfigSnapshot)
	if err != nil {
		return false, fmt.Errorf("unmarshal %q snapshot: %w", capiKubeconfigSnapshot, err)
	}

	secretName := kubeconfig.SecretName(cluster)

	for _, secret := range secrets {
		if secret.Name != secretName {
			continue
		}

		if secret.EndpointHash != endpointHash {
			return false, nil
		}

		expiresAt, err := time.Parse(time.RFC3339, secret.ExpiresAt)
		if err != nil {
			return false, nil
		}

		return time.Until(expiresAt) > capiKubeconfigRenewMargin, nil
	}

	return false, nil
}

func apiserverEndpointHash(host string, caData []byte) string {
	sum := sha256.Sum256(append([]byte(host+"\n"), caData...))
	return hex.EncodeToString(sum[:])
}

func createCAPIServiceAccount(k8sClient k8s.Client, saName string) error {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-cloud-instance-manager",
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "node-manager",
				"meta.helm.sh/release-namespace": "d8-system",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
			},
		},
	}

	_, err := k8sClient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: clusterAPINamespace,
			Annotations: map[string]string{
				"meta.helm.sh/release-name":      "node-manager",
				"meta.helm.sh/release-namespace": "d8-system",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "Helm",
			},
		},
	}

	_, err = k8sClient.CoreV1().ServiceAccounts(serviceAccount.Namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "failed to create service account")
		}
	}

	return nil
}
