/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registryswitcher

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

func KubernetesConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "apps/v1",
		Kind:              "Deployment",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{"deckhouse"},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var deployment appsv1.Deployment
			if err := sdk.FromUnstructured(obj, &deployment); err != nil {
				return nil, fmt.Errorf("failed to convert deckhouse deployment to struct: %w", err)
			}

			readyMsg, isReady := helpers.AssessDeploymentStatus(&deployment)
			ret := DeckhouseDeploymentStatus{
				IsExist:  true,
				IsReady:  isReady,
				ReadyMsg: readyMsg,
			}
			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	deployment, err := helpers.SnapshotToSingle[DeckhouseDeploymentStatus](input, name)
	if err != nil {
		// If no deployment found, it's not ready
		if err == helpers.ErrNoSnapshot {
			return Inputs{
				DeckhouseDeployment: DeckhouseDeploymentStatus{
					IsExist:  false,
					IsReady:  false,
					ReadyMsg: "Deckhouse deployment not found",
				},
			}, nil
		}
		return Inputs{}, fmt.Errorf("get deckhouse deployment snapshot error: %w", err)
	}

	return Inputs{
		DeckhouseDeployment: deployment,
	}, nil
}
