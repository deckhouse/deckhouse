/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	legacyProvisioner = `vsphere.csi.vmware.com`
	modernProvisioner = `csi.vsphere.vmware.com`
)

type StorageClass struct {
	Name        string
	Provisioner string
}

func ApplyStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	sc := &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes VolumeAttachment to VolumeAttachment: %v", err)
	}

	return StorageClass{
		Name:        sc.Name,
		Provisioner: sc.Provisioner,
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

func handleStorageClasses(_ context.Context, input *go_hook.HookInput) error {
	// We use `none` in internal values against empty string `` for cleaner conditions in Helm templates.
	compatibilityFlag := "none"
	if v, ok := input.Values.GetOk("cloudProviderVsphere.storageClass.compatibilityFlag"); ok {
		compatibilityFlag = v.String()
	}
	input.Values.Set("cloudProviderVsphere.internal.compatibilityFlag", compatibilityFlag)

	storageClasses, err := sdkobjectpatch.UnmarshalToStruct[StorageClass](input.Snapshots, "module_storageclasses")
	if err != nil {
		return fmt.Errorf("failed to unmarshal module_storageclasses snapshot: %w", err)
	}

	for _, sc := range storageClasses {
		switch compatibilityFlag {
		case "Legacy":
			if sc.Provisioner != modernProvisioner {
				continue
			}
			input.Logger.Info("Deleting storageclass because legacy one will be rolled out", slog.String("storage_class", sc.Name))
		default:
			if sc.Provisioner != legacyProvisioner {
				continue
			}
			input.Logger.Info("Deleting storageclass because modern one will be rolled out", slog.String("storage_class", sc.Name))
		}

		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", sc.Name)
	}

	return nil
}
