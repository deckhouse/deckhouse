package bashibleapiserver

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var nestedScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(nestedScheme))
	utilruntime.Must(apiregistrationv1.AddToScheme(nestedScheme))
}

func BuildNestedClient(adminKubeconfigSecret *corev1.Secret) (client.Client, error) {
	raw, ok := adminKubeconfigSecret.Data[string(kubeconfig.SuperAdmin)]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s missing key %q", adminKubeconfigSecret.Namespace, adminKubeconfigSecret.Name, kubeconfig.SuperAdmin)
	}

	restCfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse nested kubeconfig: %w", err)
	}

	return client.New(restCfg, client.Options{Scheme: nestedScheme})
}
