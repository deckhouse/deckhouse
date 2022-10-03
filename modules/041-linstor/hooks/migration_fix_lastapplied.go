/*
Copyright 2022 Flant JSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

/*
Temporary hook used for migration, it could be removed after 01.11.2022
*/

/*
Piraeus-operator prior v1.9.0 incorrectly set the last applied annotation, which makes thee way
merge patch not working for removing such keys as nodeAffinity and tolerations. In Deckhouse v1.35
we moved LINSTOR from master to system nodes, so we should to take care about all affected objects.
Thus we just remove them, piraeus-operator will create new ones with correct settings.
More details here:
- https://github.com/piraeusdatastore/piraeus-operator/issues/321
- https://github.com/piraeusdatastore/piraeus-operator/pull/323
*/

type LinstorDeploymentsSnapshot struct {
	Name          string
	Affinity      v1.Affinity
	Tollertations []v1.Toleration
}

func applyLinstorDeplotmentsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	deployment := &appsv1.Deployment{}
	err := sdk.FromUnstructured(obj, deployment)
	if err != nil {
		return nil, fmt.Errorf("cannot convert deployment: %v", err)
	}

	return &LinstorDeploymentsSnapshot{
		Name:          deployment.Name,
		Affinity:      *deployment.Spec.Template.Spec.Affinity,
		Tollertations: deployment.Spec.Template.Spec.Tolerations,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deployments",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"linstor-controller", "linstor-csi-controller"},
			},
			FilterFunc:                   applyLinstorDeplotmentsFilter,
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			WaitForSynchronization:       pointer.BoolPtr(false),
		},
	},
}, deleteWrongResources)

const (
	controlPlaneLabelKey = "node-role.kubernetes.io/control-plane"
	masterLabelKey       = "node-role.kubernetes.io/master"
)

func deleteWrongResources(input *go_hook.HookInput) error {
LOOP:
	for _, snapshot := range input.Snapshots["deployments"] {
		linstorDeploymentSnapshot := snapshot.(*LinstorDeploymentsSnapshot)
		if linstorDeploymentSnapshot.Name != "linstor-controller" && linstorDeploymentSnapshot.Name != "linstor-csi-controller" {
			continue LOOP
		}
		for _, toleration := range linstorDeploymentSnapshot.Tollertations {
			if toleration.Key == controlPlaneLabelKey || toleration.Key == masterLabelKey {
				input.PatchCollector.Delete("apps/v1", "Deployment", linstorNamespace, linstorDeploymentSnapshot.Name)
				continue LOOP
			}
		}
		if linstorDeploymentSnapshot.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			continue LOOP
		}
		for _, terms := range linstorDeploymentSnapshot.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, a := range terms.MatchExpressions {
				if a.Key == controlPlaneLabelKey || a.Key == masterLabelKey {
					input.PatchCollector.Delete("apps/v1", "Deployment", linstorNamespace, linstorDeploymentSnapshot.Name)
					continue LOOP
				}
			}
		}
	}

	return nil
}
