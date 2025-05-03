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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/shared"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/set_node_group_status_engine",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ngs",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			FilterFunc:                   filterNodeGroupEngine,
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
		},
		{
			Name:       "migration_status",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{shared.EngineMigrationSuccessfulConfigMapName},
			},
			FilterFunc:                   filterMigrationStatus,
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
		},
	},
}, setNodeGroupEngine)

func filterMigrationStatus(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// we do not need any information from this cm only that this cm exists in the cluster
	return true, nil
}

func filterNodeGroupEngine(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return shared.NodeGroupToNodeGroupEngineParam(&ng), nil
}

func setNodeGroupEngine(input *go_hook.HookInput) error {
	if len(input.Snapshots["migration_status"]) == 1 {
		input.Logger.Info("Migration already done! Skip.")
		return nil
	}

	// we need this migration only one time
	// and when cluster is bootstrapping
	// normal logic placed in get_crds hook
	// in the second case we will not have any node groups, and
	// we will create only cm that migration was passed
	// in the first case we have 4 states:
	// - clusters with Cluster API enabled only as vcd for example
	// - clusters with MCM enabled only as yandex for example
	// - clusters with MCM and CAPI enabled it is only openstack
	// - static or hybrid clusters with CAPS ngs
	// and separate case with hybrid clusters, thus we are using cloud providers
	// modules for checks, not clusterConfiguration.provider
	// For first case in status.engine we should set CAPI
	// For second and third case we should set MCM because all cloudEphemeral nod groups
	// were created by MCM
	// for fourth case we should check static node groups on existing staticInstances field
	// and set CAPI
	// for another static CloudPermanent and CloudStatic type we will set Static
	// because we are interesting with first case we check default value for engine
	// by array with cloud providers with CAPI only

	cloudEphemeralEngineDefault := ngv1.NodeGroupEngineMCM
	for _, provider := range shared.ProvidersWithCAPIOnly {
		if module.IsEnabled(provider, input) {
			input.Logger.Infof("Enabling CAPI only for provider %s", provider)
			cloudEphemeralEngineDefault = ngv1.NodeGroupEngineCAPI
			break
		}
	}

	input.Logger.Infof("Default cloud ephemeral engine is %s", cloudEphemeralEngineDefault)

	for _, ngRaw := range input.Snapshots["ngs"] {
		ng := ngRaw.(shared.NodeGroupEngineParam)
		//if ng.Engine != "" {
		//	input.Logger.Infof("Skipping node group %s engine type migration becauseit was set to %s", ng.NodeGroupName, ng.Engine)
		//	continue
		//}

		engine, err := shared.CalculateEngine(ng, cloudEphemeralEngineDefault)
		if err != nil {
			return err
		}

		input.Logger.Infof("For node group %s will set engine to %s", ng.NodeGroupName, engine)

		shared.StatusEnginePatch(input, ng.NodeGroupName, engine)
	}

	input.PatchCollector.CreateIfNotExists(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      shared.EngineMigrationSuccessfulConfigMapName,
			Namespace: "kube-system",
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
	})

	return nil
}
