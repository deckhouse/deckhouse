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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(discoverApiserverCA))

func discoverApiserverCA(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	// Take the CA from the cluster the hooks actually talk to — the managed
	// cluster when a kubeconfig is configured, the in-cluster service account
	// otherwise — instead of always reading the pod's own service-account CA.
	config, err := dc.GetClientConfig()
	if err != nil {
		return fmt.Errorf("get client config: %w", err)
	}

	if len(config.CAData) == 0 {
		return fmt.Errorf("kubernetes CA is empty")
	}

	input.Values.Set("global.discovery.kubernetesCA", string(config.CAData))
	return nil
}
