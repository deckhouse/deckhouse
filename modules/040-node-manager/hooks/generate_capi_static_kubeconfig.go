package hooks

import (
	"context"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/pkg/errors"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/kubeconfig"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/node-manager",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "capi_static_kubeconfig_secret",
			Crontab: "0 1 * * *",
		},
	},
}, dependency.WithExternalDependencies(handleCreateCAPIStaticKubeconfig))

func handleCreateCAPIStaticKubeconfig(input *go_hook.HookInput, dc dependency.Container) error {
	capiEnabledRaw := input.Values.Get("nodeManager.internal.capiControllerManagerEnabled")
	capsEnabledRaw := input.Values.Get("nodeManager.internal.capsControllerManagerEnabled")

	if capiEnabledRaw.Exists() && capiEnabledRaw.Bool() {
		capiClusterName := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
		err := generateStaticKubeconfigSecret(input, dc, hookParam{
			serviceAccount: clusterAPICloudServiceAccountName,
			cluster:        capiClusterName,
		})
		if err != nil {
			return err
		}
	}

	if capsEnabledRaw.Exists() && capsEnabledRaw.Bool() {
		err := generateStaticKubeconfigSecret(input, dc, hookParam{
			serviceAccount: clusterAPIStaticServiceAccountName,
			cluster:        clusterAPIStaticClusterName,
		})
		if err != nil {
			return err
		}
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
