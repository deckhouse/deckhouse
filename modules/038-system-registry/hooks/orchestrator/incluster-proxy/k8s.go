/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-system-registry/hooks/helpers"
)

const (
	versionAnnotation = "registry.deckhouse.io/version"
	deploymentName    = "registry-incluster-proxy"
)

func KubernetesConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "apps/v1",
		Kind:              "Deployment",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{deploymentName},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var d appsv1.Deployment
			err := sdk.FromUnstructured(obj, &d)
			if err != nil {
				return nil, fmt.Errorf("failed to convert deployment \"%s\" to struct: %v", obj.GetName(), err)
			}

			readyMsg, isReady := helpers.AssessDeploymentStatus(&d)
			ret := Inputs{
				IsExist:  true,
				IsReady:  isReady,
				ReadyMsg: readyMsg,
				Version:  d.Annotations[versionAnnotation],
			}
			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	return helpers.SnapshotToSingle[Inputs](input, name)
}
