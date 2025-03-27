/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

type BashibleConfig struct {
	ProxyEndpoints []string      `json:"proxyEndpoints"`
	Hosts          []HostsObject `json:"hosts"`
	PrepullHosts   []HostsObject `json:"prepullHosts"`
}

type HostsObject struct {
	Host    string             `json:"host"`
	Mirrors []MirrorHostObject `json:"mirrors"`
}

type MirrorHostObject struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
	CA       string `json:"ca"`
	Scheme   string `json:"scheme"`
}

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
	},
}, handleBashibleConfig)

func handleBashibleConfig(input *go_hook.HookInput) error {
	// Check module registry mode
	if !shouldRunStaticPodRegistry(getMode(input)) {
		setBashibleConfig(input, emptyBashibleConfig())
		return nil
	}

	// Extract snaps
	var (
		masterNodesIPs []string
		rPKI           RegistryPKI
		rUser          RegistryUser
	)

	{
		var multiErr multierror.Error
		masterNodesIPs = extractFromSnapNodeInternalIP(input.Snapshots["master_nodes_ips"])

		if rPKIData, err := extractFromSnapRegistryPKI(input.Snapshots["registry_pki"]); err != nil {
			multiErr.Errors = append(multiErr.Errors, err)
		} else {
			rPKI = rPKIData
		}

		if rUserData, err := extractFromSnapRegistryUser(input.Snapshots["registry_ro_user"]); err != nil {
			multiErr.Errors = append(multiErr.Errors, err)
		} else {
			rUser = rUserData
		}

		// Check if any error occurred
		if err := multiErr.ErrorOrNil(); err != nil {
			// We can't set newEmptyBashibleConfig, maybe someone accidentally deleted the secret
			// We can't return an error because it might mean the manager isn't started yet. Returning an error would prevent the manager from starting
			// We should set default config if it doesn't exist
			setBashibleConfigIfNotExist(input, emptyBashibleConfig())
			input.Logger.Error(err.Error())
			return nil
		}
	}

	proxyEndpoints := make([]string, 0, len(masterNodesIPs))
	for _, masterNodesIP := range masterNodesIPs {
		proxyEndpoints = append(proxyEndpoints, fmt.Sprintf("%s:%d", masterNodesIP, RegistryPort))
	}

	mirrors := createMirrors(rUser, rPKI, []string{RegistryProxyHost})
	prepullMirrors := createMirrors(rUser, rPKI, append([]string{RegistryProxyHost}, proxyEndpoints...))

	bashibleConfig := BashibleConfig{
		ProxyEndpoints: proxyEndpoints,
		Hosts:          []HostsObject{{Host: RegistryHost, Mirrors: mirrors}},
		PrepullHosts:   []HostsObject{{Host: RegistryHost, Mirrors: prepullMirrors}},
	}

	setBashibleConfig(input, bashibleConfig)
	return nil
}

func createMirrors(rUser RegistryUser, rPKI RegistryPKI, hosts []string) []MirrorHostObject {
	mirrors := make([]MirrorHostObject, 0, len(hosts))
	for _, host := range hosts {
		mirrors = append(mirrors, MirrorHostObject{
			Host:     host,
			Username: rUser.Name,
			Password: rUser.Password,
			Auth:     rUser.Auth(),
			CA:       rPKI.CA,
			Scheme:   RegistryScheme,
		})
	}
	return mirrors
}

func emptyBashibleConfig() BashibleConfig {
	return BashibleConfig{
		ProxyEndpoints: []string{},
		Hosts:          []HostsObject{},
		PrepullHosts:   []HostsObject{},
	}
}

func setBashibleConfig(input *go_hook.HookInput, cfg BashibleConfig) {
	input.Values.Set(inputValuesBashibleCfg, cfg)
}

func setBashibleConfigIfNotExist(input *go_hook.HookInput, cfg BashibleConfig) {
	obj := input.Values.Get(inputValuesBashibleCfg)

	if !obj.Exists() {
		input.Values.Set(inputValuesBashibleCfg, cfg)
	}
}
