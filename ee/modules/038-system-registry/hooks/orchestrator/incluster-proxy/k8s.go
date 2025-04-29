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

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	VersionAnnotation = "registry.deckhouse.io/incluster-proxy-version"
	DeploymentName    = "registry-incluster-proxy"
)

func KubernetesConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "apps/v1",
		Kind:              "Deployment",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{DeploymentName},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var d appsv1.Deployment
			err := sdk.FromUnstructured(obj, &d)
			if err != nil {
				return nil, fmt.Errorf("failed to convert deployment \"%s\" to struct: %v", obj.GetName(), err)
			}

			rolloutMsg, isRollout := getDeploymentRolloutStatus(&d)

			ret := Inputs{
				IsExist:    true,
				IsRollout:  isRollout,
				RolloutMsg: rolloutMsg,
				Version:    d.Annotations[VersionAnnotation],
			}
			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	return helpers.SnapshotToSingle[Inputs](input, name)
}

func getDeploymentRolloutStatus(deployment *appsv1.Deployment) (string, bool) {
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated...", deployment.Name, deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas), false
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination...", deployment.Name, deployment.Status.Replicas-deployment.Status.UpdatedReplicas), false
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available...", deployment.Name, deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas), false
		}
		return fmt.Sprintf("Deployment %q successfully rolled out", deployment.Name), true
	}
	return "Waiting for deployment spec update to be observed...", false
}
