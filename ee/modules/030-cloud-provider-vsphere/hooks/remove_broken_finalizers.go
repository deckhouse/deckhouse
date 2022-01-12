/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var (
	removeFinalizersPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": nil,
		},
	}
)

type VolumeAttachment struct {
	Name    string
	Message string
}

func ApplyVolumeAttachmentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	va := &storagev1.VolumeAttachment{}
	err := sdk.FromUnstructured(obj, va)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes VolumeAttachment to VolumeAttachment: %v", err)
	}

	message := ""
	if va.Status.DetachError != nil {
		message = va.Status.DetachError.Message
	}
	return VolumeAttachment{Name: va.Name, Message: message}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/cloud-provider-vsphere",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "finalizers",
			ApiVersion:                   "storage.k8s.io/v1",
			Kind:                         "VolumeAttachment",
			FilterFunc:                   ApplyVolumeAttachmentFilter,
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
		},
	},
}, handleVolumeAttachments)

func handleVolumeAttachments(input *go_hook.HookInput) error {
	snap, ok := input.Snapshots["finalizers"]
	if !ok {
		return nil
	}

	for _, s := range snap {
		va := s.(VolumeAttachment)
		if va.Message != "rpc error: code = Unknown desc = No VM found" {
			continue
		}
		input.PatchCollector.MergePatch(removeFinalizersPatch, "storage.k8s.io/v1", "VolumeAttachment", "", va.Name)
	}

	return nil
}
