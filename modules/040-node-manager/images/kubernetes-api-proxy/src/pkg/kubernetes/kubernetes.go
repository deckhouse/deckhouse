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

package kubernetes

import (
	"errors"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"kubernetes-api-proxy/internal/config"
)

const (
	caFile   = "/var/run/kubernetes.io/kubernetes-api-proxy/ca.crt"
	certFile = "/var/run/kubernetes.io/kubernetes-api-proxy/cl.crt"
	keyFile  = "/var/run/kubernetes.io/kubernetes-api-proxy/cl.key"
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
