/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	RegistryPort   = 5001
	RegistryPath   = "/system/deckhouse"
	RegistrySchema = "https"
)

var (
	RegistryHost      = fmt.Sprintf("embedded-registry.d8-system.svc:%d", RegistryPort)
	RegistryProxyHost = fmt.Sprintf("127.0.0.1:%d", RegistryPort)
	RegistryBaseHost  = fmt.Sprintf("%s%s", RegistryHost, RegistryPath)
)

type RegistryPKI struct {
	CA string
}

func (rPKI *RegistryPKI) Validate() error {
	if strings.TrimSpace(rPKI.CA) == "" {
		return fmt.Errorf("empty ca")
	}
	return nil
}

type RegistryUser struct {
	Name     string
	Password string
}

func (rUser *RegistryUser) Validate() error {
	if strings.TrimSpace(rUser.Name) == "" {
		return fmt.Errorf("empty user name")
	}
	if strings.TrimSpace(rUser.Password) == "" {
		return fmt.Errorf("empty user password")
	}
	return nil
}

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
	CA       string `json:"ca"`
	Schema   string `json:"schema"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/bashible-config",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes_ips",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: filterMasterNodeInternalIP,
		},
		{
			Name:       "registry_pki",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-pki"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: filterRegistryPKI,
		},
		{
			Name:       "registry_ro_user",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-user-ro"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: filterRegistryUser,
		},
	},
}, handleBashibleConfig)

func filterMasterNodeInternalIP(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, fmt.Errorf("failed to convert master node to struct: %w", err)
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1core.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return nil, nil
}

func filterRegistryPKI(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("failed to convert registry pki secret to struct: %w", err)
	}

	ret := RegistryPKI{CA: string(secret.Data["registry-ca.crt"])}
	if err := ret.Validate(); err != nil {
		return nil, fmt.Errorf("validation error for registry pki secret: %w", err)
	}
	return ret, nil
}

func filterRegistryUser(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("failed to convert registry user secret to struct: %w", err)
	}

	ret := RegistryUser{
		Name:     string(secret.Data["name"]),
		Password: string(secret.Data["password"]),
	}
	if err := ret.Validate(); err != nil {
		return nil, fmt.Errorf("validation error for registry user secret: %w", err)
	}
	return ret, nil
}

func handleBashibleConfig(input *go_hook.HookInput) error {
	var (
		rPKISnaps      = input.Snapshots["registry_pki"]
		rUserSnaps     = input.Snapshots["registry_ro_user"]
		masterNodesIPs = set.NewFromSnapshot(input.Snapshots["master_nodes_ips"]).Slice()
	)

	// Check module registry mode
	rMode, ok := input.Values.GetOk("systemRegistry.mode")
	if !ok {
		return fmt.Errorf("registry mode ('systemRegistry.mode') not found")
	}
	if !isAllowedMode(rMode.String()) {
		input.Values.Set("systemRegistry.internal.bashible", newEmptyBashibleConfig())
		return nil
	}

	rPKI, rUser, err := extractSecrets(rPKISnaps, rUserSnaps)
	if err != nil {
		// We can't set newEmptyBashibleConfig, maybe someone accidentally deleted the secret
		// We can't return an error because it might mean that the manager hasn't started yet. The error would prevent the manager from starting
		input.Logger.Error(err.Error())
		return nil
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

	input.Values.Set("systemRegistry.internal.bashible", bashibleConfig)
	return nil
}

func isAllowedMode(mode string) bool {
	return slices.Contains([]string{"Proxy", "Local", "Detached"}, mode)
}

func newEmptyBashibleConfig() BashibleConfig {
	return BashibleConfig{
		ProxyEndpoints: []string{},
		Hosts:          []HostsObject{},
		PrepullHosts:   []HostsObject{},
	}
}

func extractSecrets(rPKISnaps, rUserSnaps []go_hook.FilterResult) (RegistryPKI, RegistryUser, error) {
	if len(rPKISnaps) == 0 || len(rUserSnaps) == 0 {
		return RegistryPKI{}, RegistryUser{}, fmt.Errorf("registry pki or user secrets are missing")
	}

	rPKI := rPKISnaps[0].(RegistryPKI)
	rUser := rUserSnaps[0].(RegistryUser)
	return rPKI, rUser, nil
}

func createMirrors(rUser RegistryUser, rPKI RegistryPKI, hosts []string) []MirrorHostObject {
	mirrors := make([]MirrorHostObject, 0, len(hosts))
	for _, host := range hosts {
		mirrors = append(mirrors, MirrorHostObject{
			Host:     host,
			Username: rUser.Name,
			Password: rUser.Password,
			CA:       rPKI.CA,
			Schema:   RegistrySchema,
		})
	}
	return mirrors
}
