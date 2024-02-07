/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// We have to delete last release in pending state
// otherwise helm release would stuck in the pending-upgrade state and deckhouse will rollback to the previous release

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:         "releases",
			ApiVersion:   "v1",
			Kind:         "Secret",
			NameSelector: nil,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "deckhouse", "owner": "helm"},
			},
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			FilterFunc:                   filterDeckhouseHelmRelease,
		},
	},
}, pendingReleaseHandler)

func pendingReleaseHandler(input *go_hook.HookInput) error {
	var latestRelease *deckhouseHelmRelease

	snap := input.Snapshots["releases"]

	if len(snap) == 0 {
		return nil
	}

	for _, sn := range snap {
		rel := sn.(deckhouseHelmRelease)

		if latestRelease == nil || rel.CreatedAt.After(latestRelease.CreatedAt) {
			latestRelease = &rel
		}
	}

	if latestRelease.Status == "pending-install" || latestRelease.Status == "pending-upgrade" || latestRelease.Status == "pending-rollback" {
		input.PatchCollector.Delete("v1", "Secret", "d8-system", latestRelease.SecretName)
	}

	return nil
}

func filterDeckhouseHelmRelease(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return deckhouseHelmRelease{
		SecretName: obj.GetName(),
		Status:     obj.GetLabels()["status"],
		CreatedAt:  obj.GetCreationTimestamp().Time,
	}, nil
}

type deckhouseHelmRelease struct {
	SecretName string
	Status     string
	CreatedAt  time.Time
}
