/*
Copyright 2021 Flant JSC

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
	"k8s.io/client-go/tools/clientcmd"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, discoverApiserverCA)

const serviceAccountCAPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

func discoverApiserverCA(_ context.Context, input *go_hook.HookInput) error {
	ca, err := apiserverCA()
	if err != nil {
		return err
	}

	input.Values.Set("global.discovery.kubernetesCA", string(ca))
	return nil
}

func apiserverCA() ([]byte, error) {
	// When deckhouse is pointed at another cluster via a kubeconfig
	// (--kube-config/$KUBE_CONFIG, exported as $KUBECONFIG), the CA must be
	// taken from that kubeconfig: the mounted serviceaccount CA belongs to
	// the cluster hosting the deckhouse pod, not to the managed one.
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		restCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("load kubeconfig %q: %w", kubeconfigPath, err)
		}
		if len(restCfg.TLSClientConfig.CAData) > 0 {
			return restCfg.TLSClientConfig.CAData, nil
		}
		if restCfg.TLSClientConfig.CAFile != "" {
			ca, err := os.ReadFile(restCfg.TLSClientConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("read ca file from kubeconfig %q: %w", kubeconfigPath, err)
			}
			return ca, nil
		}
		return nil, fmt.Errorf("kubeconfig %q has no certificate authority", kubeconfigPath)
	}

	ca, err := os.ReadFile(serviceAccountCAPath)
	if err != nil {
		return nil, fmt.Errorf("cannot find kubernetes ca: %v, (not in pod?)", err)
	}
	return ca, nil
}
