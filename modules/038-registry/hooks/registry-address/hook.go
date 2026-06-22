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

package registryaddress

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	deckhouseregistry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	queue           = "/modules/registry/registry-address"
	hookOrder       = 15 // after the PKI hook (Order 10) so internal.pki is populated
	dhRegistrySnap  = "deckhouse-registry"
	caValuesPath    = "registry.internal.pki.ca.cert"
	usersValuesPath = "registry.internal.pki.users"
	roleReadOnly    = "ReadOnly"
)

type pkiUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: hookOrder},
		Queue:        queue,
		Kubernetes:   []go_hook.KubernetesConfig{deckhouseRegistrySnapshot(dhRegistrySnap)},
	},
	handle,
)

func handle(_ context.Context, input *go_hook.HookInput) error {
	// Legacy / TakingOver: the orchestrator owns deckhouse-registry (real upstream).
	if !helpers.IsNewArchControl(input) {
		return nil
	}

	ca, _ := helpers.GetValue[string](input, caValuesPath)
	users, _ := helpers.GetValue[[]pkiUser](input, usersValuesPath)
	ro := findReadOnly(users)
	if ca == "" || ro == nil {
		input.Logger.Warn("registry PKI not ready; deferring deckhouse-registry constant rewrite")
		return nil
	}

	constant, err := buildConstantConfig(ca, ro.Name, ro.Password)
	if err != nil {
		return fmt.Errorf("build constant deckhouse-registry config: %w", err)
	}

	cur, err := helpers.SnapshotToSingle[deckhouseregistry.Config](input, dhRegistrySnap)
	switch {
	case err == nil:
		if cur.Equal(&constant) {
			return nil // already the constant — idempotent
		}
	case errors.Is(err, helpers.ErrNoSnapshot):
		// Secret not present yet (dhctl seeds it at bootstrap). Do not create it
		// here; patch on a later reconcile once it exists.
		return nil
	default:
		return fmt.Errorf("get deckhouse-registry snapshot: %w", err)
	}

	input.Logger.Info("new arch in control; pointing deckhouse-registry at the local registry svc")
	input.PatchCollector.PatchWithMerge(
		map[string]any{"data": constant.ToBase64Map()},
		"v1", "Secret", "d8-system", "deckhouse-registry")
	return nil
}

func findReadOnly(users []pkiUser) *pkiUser {
	for i := range users {
		if users[i].Role == roleReadOnly {
			return &users[i]
		}
	}
	return nil
}
