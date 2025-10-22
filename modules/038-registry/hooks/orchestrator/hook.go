/*
Copyright 2025 Flant JSC

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

package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/checker"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/incluster-proxy"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/registry-service"
	registryswitcher "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/registry-switcher"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/users"
)

const (
	valuesPath    = "registry.internal.orchestrator"
	SubmoduleName = "orchestrator"

	configSnapName           = "config"
	stateSnapName            = "state"
	registrySecretSnapName   = "registry-secret"
	pkiSnapName              = "pki"
	secretsSnapName          = "secrets"
	usersSnapName            = "users"
	inClusterProxySnapName   = "incluster-proxy"
	registryServiceSnapName  = "registry-service"
	bashibleSnapName         = "bashible"
	registrySwitcherSnapName = "registry-switcher"
)

func getKubernetesConfigs() []go_hook.KubernetesConfig {
	ret := []go_hook.KubernetesConfig{
		{
			Name:       configSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-config"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return nil, fmt.Errorf("failed to convert config secret to struct: %v", err)
				}
				return secret, nil
			},
		},
		{
			Name:       stateSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-state"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret

				err := sdk.FromUnstructured(obj, &secret)
				if err != nil {
					return nil, fmt.Errorf("failed to convert config secret to struct: %v", err)
				}

				stateData, ok := secret.Data["state"]
				if !ok {
					return nil, nil
				}

				return stateData, nil
			},
		},
		{
			Name:              registrySecretSnapName,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry"},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret v1core.Secret
				if err := sdk.FromUnstructured(obj, &secret); err != nil {
					return nil, fmt.Errorf("failed to convert secret %q to struct: %w", obj.GetName(), err)
				}
				ret := deckhouse_registry.Config{}
				ret.FromSecretData(secret.Data)
				return ret, nil
			},
		},
		pki.KubernetsConfig(pkiSnapName),
		users.KubernetsConfig(usersSnapName),
		registryservice.KubernetsConfig(registryServiceSnapName),
		inclusterproxy.KubernetesConfig(inClusterProxySnapName),
		registryswitcher.KubernetesConfig(registrySwitcherSnapName),
	}

	ret = append(ret, bashible.KubernetesConfig(bashibleSnapName)...)
	return ret
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        "/modules/registry/orchestrator",
		Kubernetes:   getKubernetesConfigs(),
	},
	handle,
)

func handle(ctx context.Context, input *go_hook.HookInput) error {
	moduleValues := helpers.NewValuesAccessor[Values](input, valuesPath)
	values := moduleValues.Get()

	var (
		inputs Inputs
		err    error
	)

	if values.State.Mode == "" {
		input.Logger.Info("State not initialized, trying restore from secret")

		stateData, err := helpers.SnapshotToSingle[[]byte](input, stateSnapName)
		if err == nil {
			if err = yaml.Unmarshal(stateData, &values.State); err != nil {
				err = fmt.Errorf("cannot unmarhsal YAML: %w", err)
			}
		}

		if err != nil {
			input.Logger.Warn(
				"Cannot restore state from secret, will initialize new",
				"error", err,
			)
		} else {
			input.Logger.Info("State successfully restored from secret")
		}
	}

	if values.State.Mode == "" {
		values.State.Mode = registry_const.ModeUnmanaged

		input.Logger.Warn(
			"State has no mode set, will set to Unmanaged",
		)
	}

	configSecret, err := helpers.SnapshotToSingle[v1core.Secret](input, configSnapName)
	if err != nil {
		if errors.Is(err, helpers.ErrNoSnapshot) {
			moduleValues.Clear()
			return nil
		}
		return fmt.Errorf("get Config snapshot error: %w", err)
	}
	if inputs.Params, err = configFromSecret(configSecret); err != nil {
		return fmt.Errorf("failed to process config from secret %q: %w", configSecret.Name, err)
	}

	inputs.RegistrySecret, err = helpers.SnapshotToSingle[deckhouse_registry.Config](input, registrySecretSnapName)
	if err != nil {
		return fmt.Errorf("get RegistrySecret snapshot error: %w", err)
	}

	inputs.PKI, err = pki.InputsFromSnapshot(input, pkiSnapName)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get PKI snapshot error: %w", err)
	}

	inputs.Users, err = users.InputsFromSnapshot(input, usersSnapName)
	if err != nil {
		return fmt.Errorf("get Users snapshot error: %w", err)
	}

	inputs.InClusterProxy, err = inclusterproxy.InputsFromSnapshot(input, inClusterProxySnapName)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get InClusterProxy snapshots error: %w", err)
	}

	inputs.RegistryService, err = registryservice.InputsFromSnapshot(input, registryServiceSnapName)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get RegistryService snapshot error: %w", err)
	}

	inputs.Bashible, err = bashible.InputsFromSnapshot(input, bashibleSnapName)
	if err != nil {
		return fmt.Errorf("get Bashible snapshot error: %w", err)
	}

	inputs.RegistrySwitcher, err = registryswitcher.InputsFromSnapshot(input, registrySwitcherSnapName)
	if err != nil {
		return fmt.Errorf("get RegistrySwitcher snapshot error: %w", err)
	}

	inputs.CheckerStatus = checker.GetStatus(ctx, input)

	values.Hash, err = helpers.ComputeHash(inputs)
	if err != nil {
		return fmt.Errorf("cannot compute inputs hash: %w", err)
	}

	// Initialize RegistrySecret before processing
	values.State.RegistrySecret.Config = inputs.RegistrySecret

	// Load checker params
	values.State.CheckerParams = checker.GetParams(ctx, input)

	// Process the state and update internal values
	err = values.State.process(input.Logger, inputs)
	if err != nil {
		return fmt.Errorf("cannot process: %w", err)
	}
	moduleValues.Set(values)

	// Set checker params
	err = checker.SetParams(input, values.State.CheckerParams)
	if err != nil {
		return fmt.Errorf("cannot set checker params: %w", err)
	}

	// Generate expected RegistrySecret. Apply patch to update
	newRegistrySecret := values.State.RegistrySecret.Config
	if !newRegistrySecret.Equal(&inputs.RegistrySecret) {
		input.PatchCollector.PatchWithMerge(
			map[string]any{"data": newRegistrySecret.ToBase64SecretData()},
			"v1", "Secret", "d8-system", "deckhouse-registry")
	}
	return nil
}

func configFromSecret(secret v1core.Secret) (Params, error) {
	ret := Params{
		Mode:       string(secret.Data["mode"]),
		ImagesRepo: string(secret.Data["imagesRepo"]),
		UserName:   string(secret.Data["username"]),
		Password:   string(secret.Data["password"]),
		TTL:        string(secret.Data["ttl"]),
		Scheme:     string(secret.Data["scheme"]),
		Generation: secret.Generation,
		CheckMode:  registry_const.ToCheckModeType(string(secret.Data["checkMode"])),
	}

	if rawCA := secret.Data["ca"]; len(rawCA) > 0 {
		cert, err := registry_pki.DecodeCertificate(rawCA)
		if err != nil {
			return Params{}, fmt.Errorf("failed to decode CA certificate: %w", err)
		}
		ret.CA = cert
	}
	return ret, ret.Validate()
}
