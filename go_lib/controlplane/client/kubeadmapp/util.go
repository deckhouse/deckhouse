package kubeadmapp

import (
	"path/filepath"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/errors"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ToClientSet converts a KubeConfig object to a client
func ToClientSet(config *clientcmdapi.Config) (clientset.Interface, error) {
	overrides := clientcmd.ConfigOverrides{Timeout: "10s"}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &overrides).ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create API client configuration from kubeconfig")
	}

	client, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create API client")
	}
	return client, nil
}

// ClientSetFromFile returns a ready-to-use client from a kubeconfig file
func ClientSetFromFile(path string) (clientset.Interface, error) {
	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load admin kubeconfig")
	}
	return ToClientSet(config)
}

// Client returns the Client for accessing the cluster with the identity defined in admin.conf.
func MyNewKubernetesClient() (clientset.Interface, error) {
	pathAdmin := filepath.Join(constants.KubernetesDir, constants.AdminKubeConfigFileName)

	// if j.dryRun {
	// 	dryRun := apiclient.NewDryRun()
	// 	// For the dynamic dry-run client use this kubeconfig only if it exists.
	// 	// That would happen presumably after TLS bootstrap.
	// 	if _, err := os.Stat(pathAdmin); err == nil {
	// 		if err := dryRun.WithKubeConfigFile(pathAdmin); err != nil {
	// 			return nil, err
	// 		}
	// 	} else if j.tlsBootstrapCfg != nil {
	// 		if err := dryRun.WithKubeConfig(j.tlsBootstrapCfg); err != nil {
	// 			return nil, err
	// 		}
	// 	} else if j.cfg.Discovery.BootstrapToken != nil {
	// 		insecureConfig := token.BuildInsecureBootstrapKubeConfig(j.cfg.Discovery.BootstrapToken.APIServerEndpoint)
	// 		resetConfig, err := clientcmd.NewDefaultClientConfig(*insecureConfig, &clientcmd.ConfigOverrides{}).ClientConfig()
	// 		if err != nil {
	// 			return nil, errors.Wrap(err, "failed to create API client configuration from kubeconfig")
	// 		}
	// 		if err := dryRun.WithRestConfig(resetConfig); err != nil {
	// 			return nil, err
	// 		}
	// 	}

	// 	dryRun.WithDefaultMarshalFunction().
	// 		WithWriter(os.Stdout).
	// 		AppendReactor(dryRun.GetClusterInfoReactor()).
	// 		AppendReactor(dryRun.GetKubeadmConfigReactor()).
	// 		AppendReactor(dryRun.GetKubeadmCertsReactor()).
	// 		AppendReactor(dryRun.GetKubeProxyConfigReactor()).
	// 		AppendReactor(dryRun.GetKubeletConfigReactor()).
	// 		AppendReactor(dryRun.GetNodeReactor()).
	// 		AppendReactor(dryRun.PatchNodeReactor())

	// 	j.client = dryRun.FakeClient()
	// 	return j.client, nil
	// }

	client, err := ClientSetFromFile(pathAdmin)
	if err != nil {
		return nil, errors.Wrap(err, "[preflight] couldn't create Kubernetes client")
	}
	return client, nil
}
