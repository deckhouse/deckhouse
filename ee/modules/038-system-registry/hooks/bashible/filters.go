/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"encoding/base64"
	"fmt"
	"sigs.k8s.io/yaml"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
	registry_models "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models"
)

type RegistryPKI struct {
	CA string
}

func (rPKI *RegistryPKI) Validate() error {
	if strings.TrimSpace(rPKI.CA) == "" {
		return fmt.Errorf("empty CA")
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

func (rUser *RegistryUser) Auth() string {
	authRaw := fmt.Sprintf("%s:%s", rUser.Name, rUser.Password)
	return base64.StdEncoding.EncodeToString([]byte(authRaw))
}

func filterNodeInternalIP(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, fmt.Errorf("failed to convert node to struct: %w", err)
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

func filterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ret v1core.Secret
	if err := sdk.FromUnstructured(obj, &ret); err != nil {
		return nil, fmt.Errorf("failed to convert %s to secret struct: %w", obj.GetName(), err)
	}
	return ret, nil
}

func extractFromSnapRegistryUser(snaps []go_hook.FilterResult) *RegistryUser {
	if len(snaps) == 0 {
		return nil
	}
	registryUser := snaps[0].(RegistryUser)
	return &registryUser
}

func extractFromSnapRegistryPKI(snaps []go_hook.FilterResult) *RegistryPKI {
	if len(snaps) == 0 {
		return nil
	}
	registryPKI := snaps[0].(RegistryPKI)
	return &registryPKI
}

func extractFromSnapNodeInternalIP(snaps []go_hook.FilterResult) []string {
	return set.NewFromSnapshot(snaps).Slice()
}

func extractRegistryBashibleConfigFromSecret(snaps []go_hook.FilterResult) (*registry_models.BashibleConfigSecret, error) {
	if len(snaps) == 0 {
		return nil, nil
	}
	secret := snaps[0].(v1core.Secret)

	// Check if field is empty
	rawConfig := secret.Data["config"]
	if len(rawConfig) == 0 {
		return nil, nil
	}

	var ret registry_models.BashibleConfigSecret
	if err := yaml.Unmarshal(rawConfig, ret); err != nil {
		return nil, fmt.Errorf("failed to parse registry bashible config secret: %w", err)
	}
	return &ret, nil
}
