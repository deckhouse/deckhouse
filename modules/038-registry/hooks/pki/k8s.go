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

package pki

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	initsecret "github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

// initSecretAppliedAnnotation marks the dhctl-seeded registry-init secret as
// consumed, so dhctl's WaitForRegistryInitialization removes it. In the legacy
// arch the orchestrator sets it; in the orchestrator-free new arch the PKI hook
// sets it once the bootstrap CA is durably persisted in registry-module-pki.
const initSecretAppliedAnnotation = "registry.deckhouse.io/is-applied"

// InitSnap is the registry-init snapshot: the reuse State (CA + ro/rw users)
// plus whether the secret already carries the is-applied annotation.
type InitSnap struct {
	State   State `json:"state"`
	Applied bool  `json:"applied"`
}

// KubernetesConfigs builds the snapshot configs for all PKI secrets.
func KubernetesConfigs(registryPKISnap, modulePKISnap, initPKISnap string) []go_hook.KubernetesConfig {
	return []go_hook.KubernetesConfig{
		// dhctl-seeded bootstrap secret, read-only: highest-priority CA + user
		// reuse, plus the is-applied annotation so the hook can signal dhctl to
		// remove registry-init once the CA is persisted in registry-module-pki.
		initSnapshot(initPKISnap),
		// Orchestrator's secret, read-only, used to reuse CA/token during migration.
		secretSnapshot(registryPKISnap, "registry-pki", func(data map[string][]byte) (State, error) {
			return State{
				CA:    secretDataToCertModel(data, "ca"),
				Token: secretDataToCertModel(data, "token"),
			}, nil
		}),
		// This hook's own persistent store.
		secretSnapshot(modulePKISnap, "registry-module-pki", stateFromModuleSecret),
	}
}

// stateFromInitSecret decodes the dhctl-seeded registry-init secret (data.config
// = YAML initsecret.Config) into the reuse State: the bootstrap CA + ro/rw users.
func stateFromInitSecret(data map[string][]byte) (State, error) {
	raw := data["config"]
	if len(raw) == 0 {
		return State{}, nil
	}
	var cfg initsecret.Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return State{}, fmt.Errorf("registry-init: parse config: %w", err)
	}
	s := State{}
	if cfg.CA.Cert != "" && cfg.CA.Key != "" {
		s.CA = &CertModel{Cert: cfg.CA.Cert, Key: cfg.CA.Key}
	}
	if cfg.ROUser.Name != "" {
		s.Users = append(s.Users, UserModel{
			Name:         cfg.ROUser.Name,
			Password:     cfg.ROUser.Password,
			PasswordHash: cfg.ROUser.PasswordHash,
			Role:         RoleReadOnly,
		})
	}
	if cfg.RWUser.Name != "" {
		s.Users = append(s.Users, UserModel{
			Name:         cfg.RWUser.Name,
			Password:     cfg.RWUser.Password,
			PasswordHash: cfg.RWUser.PasswordHash,
			Role:         RoleReadWrite,
		})
	}
	return s, nil
}

// initSnapshot watches registry-init, decoding both the reuse State and the
// is-applied annotation into an InitSnap.
func initSnapshot(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "v1",
		Kind:              "Secret",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{"registry-init"},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var secret v1core.Secret
			if err := sdk.FromUnstructured(obj, &secret); err != nil {
				return nil, fmt.Errorf("failed to convert secret %q to struct: %w", obj.GetName(), err)
			}
			st, err := stateFromInitSecret(secret.Data)
			if err != nil {
				return nil, err
			}
			_, applied := secret.Annotations[initSecretAppliedAnnotation]
			return InitSnap{State: st, Applied: applied}, nil
		},
	}
}

func stateFromModuleSecret(data map[string][]byte) (State, error) {
	s := State{
		CA:           secretDataToCertModel(data, "ca"),
		Token:        secretDataToCertModel(data, "token"),
		Agent:        secretDataToCertModel(data, "agent"),
		Distribution: secretDataToCertModel(data, "distribution"),
		Auth:         secretDataToCertModel(data, "auth"),
	}
	users, err := usersFromJSON(data["users.json"])
	if err != nil {
		return State{}, fmt.Errorf("registry-module-pki: %w", err)
	}
	s.Users = users
	s.HTTPSecret = string(data["http-secret"])
	return s, nil
}

func secretSnapshot(name, secretName string, build func(map[string][]byte) (State, error)) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "v1",
		Kind:              "Secret",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{secretName},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var secret v1core.Secret
			if err := sdk.FromUnstructured(obj, &secret); err != nil {
				return nil, fmt.Errorf("failed to convert secret %q to struct: %w", obj.GetName(), err)
			}
			st, err := build(secret.Data)
			if err != nil {
				return nil, err
			}
			return st, nil
		},
	}
}
