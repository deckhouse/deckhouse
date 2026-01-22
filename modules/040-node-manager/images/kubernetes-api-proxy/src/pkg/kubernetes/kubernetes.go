package kubernetes

import (
	"errors"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"kubernetes-api-proxy/internal/config"
)

const (
	caFile   = "/var/run/kubernetes.io/kap/ca.crt"
	certFile = "/var/run/kubernetes.io/kap/cl.crt"
	keyFile  = "/var/run/kubernetes.io/kap/cl.key"
)

type ClusterConfigGetter func() (*rest.Config, error)

func BuildGetter(
	cfg config.Config,
	apiserversUpstreamsList ListPicker,
) ClusterConfigGetter {
	if cfg.AsStaticPod {
		return buildClusterConfigFromFile(apiserversUpstreamsList)
	}

	return getClusterConfigViaInCluster
}

func buildClusterConfigFromFile(
	apiserversUpstreamsList ListPicker,
) ClusterConfigGetter {
	return func() (*rest.Config, error) {
		host, err := apiserversUpstreamsList.PickAsString()
		if err != nil {
			return nil, err
		}

		tlsClientConfig := rest.TLSClientConfig{
			CAFile:   caFile,
			CertFile: certFile,
			KeyFile:  keyFile,
		}

		return &rest.Config{
			Host:            host,
			TLSClientConfig: tlsClientConfig,
		}, nil
	}
}

func getClusterConfigViaInCluster() (*rest.Config, error) {
	var clientConfig *rest.Config

	if ic, err := rest.InClusterConfig(); err == nil && ic != nil {
		clientConfig = ic
	} else {
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)

		if rc, err := loader.ClientConfig(); err == nil && rc != nil {
			clientConfig = rc
		}
	}

	if clientConfig == nil {
		return nil, errors.New("failed to get kubernetes client config")
	}

	return clientConfig, nil
}
