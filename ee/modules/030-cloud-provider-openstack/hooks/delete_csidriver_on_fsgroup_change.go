/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	storage "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "csidriver",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "CSIDriver",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cinder.csi.openstack.org"},
			},
			FilterFunc: applyCSIDriverFilter,
		},
	},
}, handleCSIDriverFSGroupPolicy)

func applyCSIDriverFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var driver storage.CSIDriver
	err := sdk.FromUnstructured(obj, &driver)
	if err != nil {
		return nil, fmt.Errorf("failed to convert CSIDriver object: %v", err)
	}

	if driver.Spec.FSGroupPolicy == nil {
		return string(storage.ReadWriteOnceWithFSTypeFSGroupPolicy), nil
	}
	return string(*driver.Spec.FSGroupPolicy), nil
}

func handleCSIDriverFSGroupPolicy(_ context.Context, input *go_hook.HookInput) error {
	snapshots := input.Snapshots.Get("csidriver")
	if len(snapshots) == 0 {
		return nil
	}

	desiredPolicy := input.Values.Get("cloudProviderOpenstack.csiDriver.fsGroupPolicy").String()

	currentPolicy := ""
	err := snapshots[0].UnmarshalTo(&currentPolicy)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSIDriver snapshot: %v", err)
	}

	if currentPolicy == desiredPolicy {
		return nil
	}

	input.Logger.Warn(fmt.Sprintf(
		"CSIDriver fsGroupPolicy changed from %q to %q, deleting to allow re-creation",
		currentPolicy, desiredPolicy,
	))

	input.PatchCollector.Delete(
		"storage.k8s.io/v1", "CSIDriver",
		"", "cinder.csi.openstack.org",
	)

	return nil
}
