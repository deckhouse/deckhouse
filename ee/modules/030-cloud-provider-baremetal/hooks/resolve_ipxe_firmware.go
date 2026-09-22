/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const resolvedIPXEFirmwarePath = "cloudProviderBaremetal.internal.resolvedIPXEFirmware"

type ipxeFirmwareSnapshot struct {
	Name   string                 `json:"name"`
	Direct map[string]interface{} `json:"direct"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{{
		Name:       "baremetal_ipxe_firmwares",
		ApiVersion: "deckhouse.io/v1",
		Kind:       "BareMetalIPXEFirmware",
		FilterFunc: filterIPXEFirmware,
	}},
}, resolveIPXEFirmware)

func filterIPXEFirmware(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	direct, found, err := unstructured.NestedMap(obj.Object, "spec", "direct")
	if err != nil || !found {
		return nil, err
	}
	return ipxeFirmwareSnapshot{Name: obj.GetName(), Direct: direct}, nil
}

func resolveIPXEFirmware(_ context.Context, input *go_hook.HookInput) error {
	ref, hasRef := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.ipxeFirmwareRef.name")
	_, hasTLS := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.provisioningTLS")
	if !hasRef {
		if hasTLS {
			return fmt.Errorf("provisioningTLS requires ipxeFirmwareRef")
		}
		input.Values.Remove(resolvedIPXEFirmwarePath)
		return nil
	}
	if !hasTLS {
		return fmt.Errorf("ipxeFirmwareRef requires provisioningTLS")
	}
	if _, hasRamdisk := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.ramdiskImageRef.name"); !hasRamdisk {
		return fmt.Errorf("provisioningTLS requires ramdiskImageRef")
	}
	if _, externalDHCP := input.Values.GetOk("cloudProviderBaremetal.nodes.parameters.ironic.dhcp.external"); externalDHCP {
		return fmt.Errorf("ipxeFirmwareRef is only supported with dhcp.internal")
	}

	images, err := sdkobjectpatch.UnmarshalToStruct[ipxeFirmwareSnapshot](input.Snapshots, "baremetal_ipxe_firmwares")
	if err != nil {
		return fmt.Errorf("unmarshal BareMetalIPXEFirmware snapshots: %w", err)
	}
	for _, image := range images {
		if image.Name != ref.String() {
			continue
		}
		resolved := map[string]interface{}{}
		for _, architecture := range []string{"bios", "uefiX86_64", "uefiArm64"} {
			firmware, found, err := unstructured.NestedMap(image.Direct, architecture)
			if err != nil {
				return fmt.Errorf("read BareMetalIPXEFirmware %q %s: %w", image.Name, architecture, err)
			}
			if !found {
				continue
			}
			url, checksum := stringValueOrDefault(firmware, "url", ""), stringValueOrDefault(firmware, "sha256", "")
			if !strings.HasPrefix(url, "https://") || len(checksum) != 64 {
				return fmt.Errorf("BareMetalIPXEFirmware %q has invalid %s source", image.Name, architecture)
			}
			resolved[architecture] = map[string]interface{}{"url": url, "sha256": checksum}
		}
		if _, ok := resolved["bios"]; !ok {
			return fmt.Errorf("BareMetalIPXEFirmware %q has no BIOS firmware", image.Name)
		}
		if _, ok := resolved["uefiX86_64"]; !ok {
			return fmt.Errorf("BareMetalIPXEFirmware %q has no x86_64 UEFI firmware", image.Name)
		}
		input.Values.Set(resolvedIPXEFirmwarePath, resolved)
		return nil
	}
	return fmt.Errorf("BareMetalIPXEFirmware %q not found", ref.String())
}
