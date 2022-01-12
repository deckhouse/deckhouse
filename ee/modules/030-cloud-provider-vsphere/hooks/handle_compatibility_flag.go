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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type StorageClass struct {
	Name     string
	IsLegacy bool
	IsModern bool
}

func ApplyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sc := &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes VolumeAttachment to VolumeAttachment: %v", err)
	}

	return StorageClass{
		Name:     sc.Name,
		IsLegacy: sc.Provisioner == "vsphere.csi.vmware.com",
		IsModern: sc.Provisioner == "csi.vsphere.vmware.com",
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_storageclasses",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			FilterFunc: ApplyStorageClassFilter,
		},
	},
}, handleStorageClasses)

func handleStorageClasses(input *go_hook.HookInput) error {
	// We use `none` in internal values against empty string `` for cleaner conditions in Helm templates.
	compatibilityFlag := "none"
	if v, ok := input.Values.GetOk("cloudProviderVsphere.storageClass.compatibilityFlag"); ok {
		compatibilityFlag = v.String()
	}
	input.Values.Set("cloudProviderVsphere.internal.compatibilityFlag", compatibilityFlag)

	snap, ok := input.Snapshots["module_storageclasses"]
	if !ok {
		return nil
	}
	for _, s := range snap {
		sc := s.(StorageClass)
		if compatibilityFlag == "legacy" {
			if sc.IsLegacy {
				continue
			}
			input.LogEntry.Infof("Deleting storageclass/%s because legacy one will be rolled out", sc.Name)
		} else {
			if sc.IsModern {
				continue
			}
			input.LogEntry.Infof("Deleting storageclass/%s because modern one will be rolled out", sc.Name)
		}
		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", sc.Name)
	}

	return nil
}
