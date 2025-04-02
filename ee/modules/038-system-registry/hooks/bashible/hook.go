/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	registry_models "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/bashible-config",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:          "master_nodes_ips",
			ApiVersion:    "v1",
			Kind:          "Node",
			LabelSelector: masterNodeLabelSelector,
			FilterFunc:    filterNodeInternalIP,
		},
		{
			Name:       "registry_pki",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-pki"},
			},
			NamespaceSelector: namespaceSelector,
			FilterFunc:        filterRegistryPKI,
		},
		{
			Name:       "registry_ro_user",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-user-ro"},
			},
			NamespaceSelector: namespaceSelector,
			FilterFunc:        filterRegistryUser,
		},
		{
			Name:       "registry_bashible_config_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-bashible-config"},
			},
			NamespaceSelector: namespaceSelector,
			FilterFunc:        filterSecret,
		},
	},
}, handleBashibleConfig)

func handleBashibleConfig(input *go_hook.HookInput) error {
	// Check module registry mode

	rMode := getMode(input)
	if !registry_const.ShouldRunStaticPodRegistry(rMode) {
		removeBashibleConfig(input)
		return nil
	}

	// Extract snaps
	masterNodesIPs := extractFromSnapNodeInternalIP(input.Snapshots["master_nodes_ips"])
	rPKI := extractFromSnapRegistryPKI(input.Snapshots["registry_pki"])
	rUser := extractFromSnapRegistryUser(input.Snapshots["registry_ro_user"])

	if rPKI == nil || rUser == nil {
		// User or PKI are missing.
		// We cannot set a new empty bashible config because the secrets might have been accidentally deleted.
		// We can't return an error here, as it might indicate that the manager hasn't fully started yet.
		// Returning an error could prevent the manager from starting.
		// Instead, attempt to restore the current configuration if it exists.
		input.Logger.Warn("Registry user secrets or registry PKI secrets are missing. Attempting to restore bashible config from secret.")

		// Try to extract the current bashible configuration from the secrets
		currentBashibleConfig, err := extractRegistryBashibleConfigFromSecret(input.Snapshots["registry_bashible_config_secret"])
		if err != nil {
			return fmt.Errorf("failed to extract bashible config from secret: %w", err)
		}

		// If we successfully obtained the current configuration, set it if it doesn't already exist
		if currentBashibleConfig != nil {
			setBashibleConfigIfNotExist(input, *currentBashibleConfig)
		}
		return nil
	}

	proxyEndpoints := make([]string, 0, len(masterNodesIPs))
	for _, masterNodesIP := range masterNodesIPs {
		proxyEndpoints = append(proxyEndpoints, fmt.Sprintf("%s:%d", masterNodesIP, registry_const.Port))
	}

	mirrors := createMirrors(*rUser, []string{registry_const.ProxyHost})
	prepullMirrors := createMirrors(*rUser, append([]string{registry_const.ProxyHost}, proxyEndpoints...))
	CA := []string{}
	if rPKI.CA != "" {
		CA = append(CA, rPKI.CA)
	}

	bashibleConfig := registry_models.BashibleConfigSecret{
		Mode:           rMode,
		Version:        registry_const.DefaultVersion, // TODO:
		ImagesBase:     registry_const.HostWithPath,
		ProxyEndpoints: proxyEndpoints,
		Hosts:          []registry_models.HostsObject{{Host: registry_const.Host, CA: CA, Mirrors: mirrors}},
		PrepullHosts:   []registry_models.HostsObject{{Host: registry_const.Host, CA: CA, Mirrors: prepullMirrors}},
	}

	setBashibleConfig(input, bashibleConfig)
	return nil
}

func createMirrors(rUser RegistryUser, hosts []string) []registry_models.MirrorHostObject {
	mirrors := make([]registry_models.MirrorHostObject, 0, len(hosts))
	for _, host := range hosts {
		mirrors = append(mirrors, registry_models.MirrorHostObject{
			Host:     host,
			Username: rUser.Name,
			Password: rUser.Password,
			Auth:     rUser.Auth(),
			Scheme:   registry_const.Scheme,
		})
	}
	return mirrors
}

func removeBashibleConfig(input *go_hook.HookInput) {
	obj := input.Values.Get(inputValuesBashibleCfg)

	if obj.Exists() {
		input.Values.Remove(inputValuesBashibleCfg)
	}
}

func setBashibleConfig(input *go_hook.HookInput, cfg registry_models.BashibleConfigSecret) {
	input.Values.Set(inputValuesBashibleCfg, cfg)
}

func setBashibleConfigIfNotExist(input *go_hook.HookInput, cfg registry_models.BashibleConfigSecret) {
	obj := input.Values.Get(inputValuesBashibleCfg)

	if !obj.Exists() {
		input.Values.Set(inputValuesBashibleCfg, cfg)
	}
}
