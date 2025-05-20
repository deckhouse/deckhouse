/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/incluster-proxy"
	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-service"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

const (
	valuesPath    = "systemRegistry.internal.orchestrator"
	SubmoduleName = "orchestrator"

	configSnapName          = "config"
	stateSnapName           = "state"
	registrySecretSnapName  = "registry-secret"
	pkiSnapName             = "pki"
	secretsSnapName         = "secrets"
	usersSnapName           = "users"
	nodeServicesSnapName    = "node-services"
	inClusterProxySnapName  = "incluster-proxy"
	registryServiceSnapName = "registry-service"
	bashibleSnapName        = "bashible"
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

				config := Params{
					Mode:       string(secret.Data["mode"]),
					ImagesRepo: string(secret.Data["imagesRepo"]),
					UserName:   string(secret.Data["username"]),
					Password:   string(secret.Data["password"]),
					TTL:        string(secret.Data["ttl"]),
					Scheme:     string(secret.Data["scheme"]),
					CA:         string(secret.Data["ca"]),
					Generation: secret.Generation,
				}

				return config, nil
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
	}

	ret = append(ret, nodeservices.KubernetsConfig(nodeServicesSnapName)...)
	ret = append(ret, bashible.KubernetesConfig(bashibleSnapName)...)
	return ret
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/orchestrator",
	Kubernetes:   getKubernetesConfigs(),
},
	handle,
)

func handle(input *go_hook.HookInput) error {
	moduleValues := helpers.NewValuesAccessor[Values](input, valuesPath)
	values := moduleValues.Get()

	var (
		inputs Inputs
		err    error
	)

	if values.State.ActualParams.Mode == "" {
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

			values.State = State{
				ActualParams: Params{
					Mode: registry_const.ModeUnmanaged,
				},
			}
		} else {
			input.Logger.Info("State successfully restored from secret")
		}
	}

	inputs.Params, err = helpers.SnapshotToSingle[Params](input, configSnapName)
	if err != nil {
		if errors.Is(err, helpers.ErrNoSnapshot) {
			moduleValues.Clear()
			return nil
		}

		return fmt.Errorf("get Config snapshot error: %w", err)
	}

	inputs.RegistrySecret, err = helpers.SnapshotToSingle[deckhouse_registry.Config](input, registrySecretSnapName)
	if err != nil {
		return fmt.Errorf("get RegistrySecret snapshot error: %w", err)
	}

	ingressClientCA, exists := helpers.GetIngressClientCAFromGlobalValues(input)
	if !exists {
		return fmt.Errorf("get Ingress client CA value error: CA is empty")
	}
	inputs.IngressClientCA = ingressClientCA

	inputs.PKI, err = pki.InputsFromSnapshot(input, pkiSnapName)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get PKI snapshot error: %w", err)
	}

	inputs.Users, err = users.InputsFromSnapshot(input, usersSnapName)
	if err != nil {
		return fmt.Errorf("get Users snapshot error: %w", err)
	}

	inputs.NodeServices, err = nodeservices.InputsFromSnapshot(input, nodeServicesSnapName)
	if err != nil {
		return fmt.Errorf("get NodeServices snapshots error: %w", err)
	}

	inputs.InClusterProxy, err = inclusterproxy.InputsFromSnapshot(input, inClusterProxySnapName)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get InClusterProxy snapshots error: %w", err)
	}

	inputs.RegistryService, err = registryservice.InputsFromSnapshot(input, registryServiceSnapName)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return fmt.Errorf("get PKI snapshot error: %w", err)
	}

	inputs.Bashible, err = bashible.InputsFromSnapshot(input, bashibleSnapName)
	if err != nil {
		return fmt.Errorf("get Bashible snapshot error: %w", err)
	}

	values.Hash, err = helpers.ComputeHash(inputs)
	if err != nil {
		return fmt.Errorf("cannot compute inputs hash: %w", err)
	}

	err = values.State.process(input.Logger, input.PatchCollector, inputs)
	if err != nil {
		return fmt.Errorf("cannot process: %w", err)
	}

	moduleValues.Set(values)
	return nil
}
