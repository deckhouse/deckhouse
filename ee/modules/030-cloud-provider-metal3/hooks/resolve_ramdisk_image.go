/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const resolvedRamdiskImagePath = "cloudProviderMetal3.internal.ramdiskImage"

type ramdiskImageSnapshot struct {
	Name   string             `json:"name"`
	Direct ramdiskImageDirect `json:"direct"`
}

type ramdiskImageDirect struct {
	Architecture string `json:"architecture"`
	KernelURL    string `json:"kernelURL"`
	InitramfsURL string `json:"initramfsURL"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "metal3_ramdisk_images",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Metal3RamdiskImage",
			FilterFunc: filterRamdiskImage,
		},
	},
}, resolveRamdiskImage)

func filterRamdiskImage(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	direct, found, err := unstructured.NestedMap(obj.Object, "spec", "direct")
	if err != nil {
		return nil, fmt.Errorf("read Metal3RamdiskImage %q spec.direct: %w", obj.GetName(), err)
	}
	if !found {
		return nil, nil
	}

	return ramdiskImageSnapshot{
		Name: obj.GetName(),
		Direct: ramdiskImageDirect{
			Architecture: stringValueOrDefault(direct, "architecture", "x86_64"),
			KernelURL:    stringValueOrDefault(direct, "kernelURL", ""),
			InitramfsURL: stringValueOrDefault(direct, "initramfsURL", ""),
		},
	}, nil
}

func resolveRamdiskImage(_ context.Context, input *go_hook.HookInput) error {
	ref, ok := input.Values.GetOk("cloudProviderMetal3.nodes.parameters.ironic.ramdiskImageRef.name")
	if !ok || ref.String() == "" {
		input.Values.Remove(resolvedRamdiskImagePath)
		return nil
	}

	images, err := sdkobjectpatch.UnmarshalToStruct[ramdiskImageSnapshot](input.Snapshots, "metal3_ramdisk_images")
	if err != nil {
		return fmt.Errorf("unmarshal Metal3RamdiskImage snapshots: %w", err)
	}

	for _, image := range images {
		if image.Name != ref.String() {
			continue
		}
		if image.Direct.KernelURL == "" || image.Direct.InitramfsURL == "" {
			return fmt.Errorf("Metal3RamdiskImage %q has incomplete spec.direct", image.Name)
		}
		input.Values.Set(resolvedRamdiskImagePath, map[string]interface{}{
			"direct": map[string]interface{}{
				"architecture": image.Direct.Architecture,
				"kernelURL":    image.Direct.KernelURL,
				"initramfsURL": image.Direct.InitramfsURL,
			},
		})
		return nil
	}

	return fmt.Errorf("Metal3RamdiskImage %q not found", ref.String())
}

func stringValueOrDefault(values map[string]interface{}, key, fallback string) string {
	value, ok := values[key].(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}
