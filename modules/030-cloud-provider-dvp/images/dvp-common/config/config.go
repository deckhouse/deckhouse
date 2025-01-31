/*
Copyright 2025 Flant JSC

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

package config

import (
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	envDVPKubernetesConfigBase64 = "DVP_KUBERNETES_CONFIG_BASE64"
	envDVPNamespace              = "DVP_NAMESPACE"
)

type CloudConfig struct {
	KubernetesConfigBase64 string `json:"kubernetes_config_base64"`
	Namespace              string `json:"namespace"`
}

func NewCloudConfig() (*CloudConfig, error) {
	cloudConfig := &CloudConfig{}
	kubernetecConfigBase64 := os.Getenv(envDVPKubernetesConfigBase64)
	if kubernetecConfigBase64 == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDVPKubernetesConfigBase64)
	}
	cloudConfig.KubernetesConfigBase64 = kubernetecConfigBase64

	namespace := os.Getenv(envDVPNamespace)
	if namespace == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDVPNamespace)
	}
	cloudConfig.Namespace = namespace

	return cloudConfig, nil
}

func (c *CloudConfig) GetKubernetesClientConfig() (*rest.Config, error) {
	kubeConfig, err := clientcmd.NewClientConfigFromBytes([]byte(c.KubernetesConfigBase64))
	if err != nil {
		return nil, fmt.Errorf("unable to load kubernetes config: %w", err)
	}

	clientConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get kubernetes client config: %w", err)
	}

	return clientConfig, nil
}
