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
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var kindToVersion = map[string]string{
	"vcdinstanceclass":         "deckhouse.io/v1",
	"zvirtinstanceclass":       "deckhouse.io/v1",
	"dynamixinstanceclass":     "deckhouse.io/v1",
	"huaweicloudinstanceclass": "deckhouse.io/v1",
	"dvpinstanceclass":         "deckhouse.io/v1alpha1",
}

var setInstanceClassNGUsageConfig = &go_hook.HookConfig{
	Queue: "/modules/node-manager/update_instance_class_ng",
	Kubernetes: []go_hook.KubernetesConfig{
		// A binding with dynamic kind has index 0 for simplicity.
		{
			Name:                "ics",
			ApiVersion:          "",
			Kind:                "",
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          applyUsedInstanceClassFilter,
		},
		{
			Name:                   "ngs",
			Kind:                   "NodeGroup",
			ApiVersion:             "deckhouse.io/v1",
			WaitForSynchronization: ptr.To(false),
			FilterFunc:             filterCloudEphemeralNG,
		},
		{
			Name:                         "cloud_provider_secret",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-node-manager-cloud-provider"},
			},
			FilterFunc: applyCloudProviderSecretKindZonesFilter,
		},
	},
}

var _ = sdk.RegisterFunc(setInstanceClassNGUsageConfig, setInstanceClassUsage)

func filterCloudEphemeralNG(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	if ng.Spec.NodeType != ngv1.NodeTypeCloudEphemeral {
		return nil, nil
	}

	return ngUsedInstanceClass{
		UsedInstanceClass: usedInstanceClass{
			Kind: ng.Spec.CloudInstances.ClassReference.Kind,
			Name: ng.Spec.CloudInstances.ClassReference.Name,
		},
		NodeGroupName: ng.Name,
	}, nil
}

func setInstanceClassUsage(input *go_hook.HookInput) error {
	// dynamic InstanceClass binding
	{
		kindInUse, kindFromSecret := detectInstanceClassKind(input, setInstanceClassNGUsageConfig)

		// Kind is changed, so objects in "dynamic-kind" can be ignored. Update kind and stop the hook.
		if kindInUse != kindFromSecret {
			if kindFromSecret == "" {
				input.Logger.Info("InstanceClassKind has changed to '': disable binding 'ics'", slog.String("from", kindInUse))
				*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
					Name:       "ics",
					Action:     "Disable",
					Kind:       "",
					ApiVersion: "",
				})
			} else {
				input.Logger.Info("InstanceClassKind has changed: update kind for binding 'ics'", slog.String("from", kindInUse), slog.String("to", kindFromSecret))
				*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
					Name:       "ics",
					Action:     "UpdateKind",
					Kind:       kindFromSecret,
					ApiVersion: "deckhouse.io/v1",
				})
			}
			// Save new kind as current kind.
			setInstanceClassNGUsageConfig.Kubernetes[0].Kind = kindFromSecret
			// Binding changed, hook will be restarted with new objects in "ics" snapshot.
			return nil
		}
	} // end dynamic

	icNodeConsumers := make(map[usedInstanceClass][]string)

	snaps := input.NewSnapshots.Get("ngs")
	for usedIC, err := range sdkobjectpatch.SnapshotIter[ngUsedInstanceClass](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ngs' snapshots: %w", err)
		}

		icNodeConsumers[usedIC.UsedInstanceClass] = append(icNodeConsumers[usedIC.UsedInstanceClass], usedIC.NodeGroupName)
	}

	// find instanceClasses which were unbound from NG (or ng deleted)
	snaps = input.NewSnapshots.Get("ics")
	for icm, err := range sdkobjectpatch.SnapshotIter[usedInstanceClassWithConsumers](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ics' snapshots: %w", err)
		}

		// if not found in NGs - remove consumers
		if _, ok := icNodeConsumers[icm.UsedInstanceClass]; !ok {
			icNodeConsumers[icm.UsedInstanceClass] = []string{}
		}
	}

	for ic, ngNames := range icNodeConsumers {
		statusPatch := map[string]interface{}{
			"status": map[string]interface{}{
				"nodeGroupConsumers": ngNames,
			},
		}

		apiVersion := "deckhouse.io/v1"
		// instance class can be v1alpha1 for example
		if v, ok := kindToVersion[strings.ToLower(ic.Kind)]; ok {
			apiVersion = v
		}

		input.PatchCollector.PatchWithMerge(statusPatch, apiVersion, ic.Kind, "", ic.Name, object_patch.WithIgnoreMissingObject())
	}

	return nil
}

type usedInstanceClass struct {
	Kind string
	Name string
}

type usedInstanceClassWithConsumers struct {
	UsedInstanceClass  usedInstanceClass
	NodeGroupConsumers []string
}

type ngUsedInstanceClass struct {
	UsedInstanceClass usedInstanceClass
	NodeGroupName     string
}

func applyUsedInstanceClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeGroupConsumers, ok, err := unstructured.NestedStringSlice(obj.Object, "status", "nodeGroupConsumers")
	if err != nil {
		return nil, err
	}

	if !ok {
		nodeGroupConsumers = make([]string, 0)
	}

	return usedInstanceClassWithConsumers{
		UsedInstanceClass: usedInstanceClass{
			Kind: obj.GetKind(),
			Name: obj.GetName(),
		},
		NodeGroupConsumers: nodeGroupConsumers,
	}, nil
}
