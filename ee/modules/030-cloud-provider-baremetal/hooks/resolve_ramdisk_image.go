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

const (
	defaultRamdiskImageName  = "baremetal-default-ramdisk"
	resolvedRamdiskImagePath = "cloudProviderBaremetal.internal.resolvedRamdiskImage"
)

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
			Name:       "baremetal_ramdisk_images",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "BareMetalRamdiskImage",
			FilterFunc: filterRamdiskImage,
		},
	},
}, resolveRamdiskImage)

func filterRamdiskImage(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	direct, found, err := unstructured.NestedMap(obj.Object, "spec", "direct")
	if err != nil {
		return nil, fmt.Errorf("read BareMetalRamdiskImage %q spec.direct: %w", obj.GetName(), err)
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
	if _, external := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.externalInstance"); external {
		input.Values.Remove(resolvedRamdiskImagePath)
		return nil
	}

	refName := defaultRamdiskImageName
	customRef := false
	if ref, ok := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.ramdiskImageRef.name"); ok && ref.String() != "" {
		refName = ref.String()
		customRef = true
	}

	images, err := sdkobjectpatch.UnmarshalToStruct[ramdiskImageSnapshot](input.Snapshots, "baremetal_ramdisk_images")
	if err != nil {
		return fmt.Errorf("unmarshal BareMetalRamdiskImage snapshots: %w", err)
	}

	for _, image := range images {
		if image.Name != refName {
			continue
		}
		if image.Direct.KernelURL == "" || image.Direct.InitramfsURL == "" {
			return fmt.Errorf("BareMetalRamdiskImage %q has incomplete spec.direct", image.Name)
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

	if !customRef {
		provisioningIP, ok := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.provisioningNetwork.ipAddress")
		if ok && provisioningIP.String() != "" {
			baseURL := fmt.Sprintf("http://%s:6180/images", provisioningIP.String())
			input.Values.Set(resolvedRamdiskImagePath, map[string]interface{}{
				"direct": map[string]interface{}{
					"architecture": "x86_64",
					"kernelURL":    baseURL + "/ironic-python-agent.kernel",
					"initramfsURL": baseURL + "/ironic-python-agent.initramfs",
				},
			})
			return nil
		}
	}

	return fmt.Errorf("BareMetalRamdiskImage %q not found", refName)
}

func stringValueOrDefault(values map[string]interface{}, key, fallback string) string {
	value, ok := values[key].(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}
