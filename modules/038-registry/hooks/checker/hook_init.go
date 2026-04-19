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

package checker

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	stateSnapName = "state"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/registry/checker/init",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                stateSnapName,
				ExecuteHookOnEvents: go_hook.Bool(false),
				ApiVersion:          "v1",
				Kind:                "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry-checker-state"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-system"},
					},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					var secret corev1.Secret

					err := sdk.FromUnstructured(obj, &secret)
					if err != nil {
						return nil, fmt.Errorf("failed to convert config secret to struct: %v", err)
					}

					data := stateSecretData{
						Params: secret.Data["params"],
						State:  secret.Data["state"],
					}

					return data, nil
				},
			},
		},
	},
	func(_ context.Context, input *go_hook.HookInput) error {
		if input.Values.Get(valuesInitializedPath).Bool() {
			return nil
		}

		input.Logger.Info("Checker state not initialized, trying restore from secret")

		var (
			state  stateModel
			params Params
		)

		stateData, err := helpers.SnapshotToSingle[stateSecretData](input, stateSnapName)
		if err == nil {
			if len(stateData.State) > 0 {
				if err = yaml.Unmarshal(stateData.State, &state); err != nil {
					input.Logger.Warn(
						"Cannot unmarshal state data from secret",
						"error", err,
					)
				}
			}

			if len(stateData.Params) > 0 {
				if err = yaml.Unmarshal(stateData.Params, &params); err != nil {
					input.Logger.Warn(
						"Cannot unmarshal params data from secret",
						"error", err,
					)
				} else {
					if err = params.Validate(); err != nil {
						input.Logger.Warn(
							"Cannot validate params data from secret",
							"error", err,
						)

						params = Params{}
					}
				}
			}

			input.Logger.Info("State successfully restored from secret")
		} else {
			input.Logger.Warn(
				"Cannot restore state from secret, will initialize new",
				"error", err,
			)
		}

		stateAccessor := helpers.NewValuesAccessor[stateModel](input, valuesStatePath)
		paramsAccessor := helpers.NewValuesAccessor[Params](input, valuesParamsPath)

		stateAccessor.Set(state)
		paramsAccessor.Set(params)

		input.Values.Set(valuesInitializedPath, true)
		return nil
	},
)
