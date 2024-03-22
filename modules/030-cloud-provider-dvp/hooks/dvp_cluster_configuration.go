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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
)

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, secretFound bool) error {
	if !secretFound {
		return fmt.Errorf("kube-system/d8-provider-cluster-configuration secret not found")
	}

	input.Values.Set("cloudProviderDvp.internal.providerClusterConfiguration", metaCfg.ProviderClusterConfig)
	input.Values.Set("cloudProviderDvp.internal.providerDiscoveryData", providerDiscoveryData.Object)
	return nil
})
