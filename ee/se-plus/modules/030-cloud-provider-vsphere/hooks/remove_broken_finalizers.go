/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type volumeAttachment struct {
	Name    string
	Message string
}

func applyVolumeAttachmentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	va := &storagev1.VolumeAttachment{}
	err := sdk.FromUnstructured(obj, va)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes VolumeAttachment to VolumeAttachment: %v", err)
	}

	message := ""
	if va.Status.DetachError != nil {
		message = va.Status.DetachError.Message
	}
	return volumeAttachment{Name: va.Name, Message: message}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/cloud-provider-vsphere",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "finalizers",
			ApiVersion:                   "storage.k8s.io/v1",
			Kind:                         "VolumeAttachment",
			FilterFunc:                   applyVolumeAttachmentFilter,
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
}, handleVolumeAttachments)

func handleVolumeAttachments(_ context.Context, input *go_hook.HookInput) error {
	snap, err := sdkobjectpatch.UnmarshalToStruct[volumeAttachment](input.Snapshots, "finalizers")
	if err != nil {
		return fmt.Errorf("failed to unmarshal finalizers snapshot: %w", err)
	}
	if len(snap) == 0 {
		return nil
	}

	for _, va := range snap {
		if va.Message != "rpc error: code = Unknown desc = No VM found" {
			continue
		}

		input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var v storagev1.VolumeAttachment
			err := sdk.FromUnstructured(obj, &v)
			if err != nil {
				return nil, err
			}
			v.ObjectMeta.Finalizers = nil
			return sdk.ToUnstructured(&v)
		}, "storage.k8s.io/v1", "VolumeAttachment", "", va.Name)
	}

	return nil
}
