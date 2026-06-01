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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue + "/migrate_cpo_cpn_to_namespaced",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cpn_crd",
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"controlplanenodes.control-plane.deckhouse.io"},
			},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterCRDScopeIsCluster,
		},
		{
			Name:       "cpo_crd",
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"controlplaneoperations.control-plane.deckhouse.io"},
			},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterCRDScopeIsCluster,
		},
	},
}, migrateCPOCPNToNamespaced)

func filterCRDScopeIsCluster(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	scope, _, _ := unstructured.NestedString(obj.Object, "spec", "scope")
	if scope != "Cluster" {
		return nil, nil
	}
	return obj.GetName(), nil
}

func migrateCPOCPNToNamespaced(_ context.Context, input *go_hook.HookInput) error {
	for _, snapName := range []string{"cpn_crd", "cpo_crd"} {
		for _, snap := range input.Snapshots.Get(snapName) {
			var crdName string
			if err := snap.UnmarshalTo(&crdName); err != nil {
				return err
			}
			input.Logger.Info("deleting cluster-scoped CRD to migrate to namespaced scope", "crd", crdName)
			input.PatchCollector.Delete("apiextensions.k8s.io/v1", "CustomResourceDefinition", "", crdName)
		}
	}
	return nil
}
