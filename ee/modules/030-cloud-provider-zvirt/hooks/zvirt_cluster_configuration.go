/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
	input.Values.Set("cloudProviderZvirt.internal.providerClusterConfiguration", metaCfg.ProviderClusterConfig)
	input.Values.Set("cloudProviderZvirt.internal.providerDiscoveryData", providerDiscoveryData.Object)
	return nil
})
