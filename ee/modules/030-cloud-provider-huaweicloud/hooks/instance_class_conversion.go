/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "huaweicloud_instance_classes",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "HuaweiCloudInstanceClass",
			FilterFunc: passInstanceClass,
		},
	},
}, handleInstanceClassConversion)

func passInstanceClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

func handleInstanceClassConversion(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("huaweicloud_instance_classes")
	for _, snap := range snaps {
		u := new(unstructured.Unstructured)
		if err := snap.UnmarshalTo(u); err != nil {
			return fmt.Errorf("convert snapshot to unstructured: %w", err)
		}

		spec, ok := u.Object["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		changed := false

		if raw, exists := spec["subnets"]; exists && raw != nil {
			subnets, isList := toStringSlice(raw)
			if isList && len(subnets) > 0 {
				if _, hasMain := spec["mainNetwork"]; !hasMain || isEmptyString(spec["mainNetwork"]) {
					spec["mainNetwork"] = subnets[0]
					changed = true
				}

				addN := []string{}
				if ex, ok := spec["additionalNetworks"]; ok && ex != nil {
					addN, _ = toStringSlice(ex)
				}
				merged := dedupPreserveOrder(append(addN, subnets[1:]...))
				spec["additionalNetworks"] = toIfaceSlice(merged)
				changed = true

				spec["subnets"] = nil
			}
		}

		if !changed {
			continue
		}

		patchObj := map[string]interface{}{
			"spec": spec,
		}
		patchBytes, err := json.Marshal(patchObj)
		if err != nil {
			return fmt.Errorf("marshal patch for %s: %w", u.GetName(), err)
		}

		target := &unstructured.Unstructured{}
		target.SetAPIVersion("deckhouse.io/v1")
		target.SetKind("HuaweiCloudInstanceClass")
		target.SetName(u.GetName())

		input.PatchCollector.PatchWithMerge(patchBytes, "deckhouse.io/v1", "HuaweiCloudInstanceClass", "", u.GetName())
	}

	return nil
}

func toStringSlice(v interface{}) ([]string, bool) {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, el := range t {
			switch s := el.(type) {
			case string:
				out = append(out, s)
			default:
			}
		}
		return out, true
	case []string:
		return t, true
	case string:
		return []string{t}, true
	default:
		return nil, false
	}
}

func toIfaceSlice(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func isEmptyString(v interface{}) bool {
	s, ok := v.(string)
	return !ok || s == ""
}

func dedupPreserveOrder(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
