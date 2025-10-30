/*
Copyright 2021 Flant JSC

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
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	huaweiInstanceClassAPIVersion = "deckhouse.io/v1"
)

type instanceClass struct {
	Name string
	Spec map[string]interface{}
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-huaweicloud/convert_instance_class_subnets",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "huaweicloud_instance_classes",
			ApiVersion:                   huaweiInstanceClassAPIVersion,
			Kind:                         "HuaweiCloudInstanceClass",
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				spec, _ := obj.Object["spec"].(map[string]interface{})
				return instanceClass{Name: obj.GetName(), Spec: spec}, nil
			},
		},
	},
}, handleHuaweiCloudInstanceClassConversion)

func handleHuaweiCloudInstanceClassConversion(_ context.Context, input *go_hook.HookInput) error {
	for ic, err := range sdkobjectpatch.SnapshotIter[instanceClass](input.Snapshots.Get("huaweicloud_instance_classes")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over HuaweiCloudInstanceClass snapshots: %w", err)
		}
		if ic.Spec == nil {
			continue
		}

		subnetsRaw, ok := ic.Spec["subnets"]
		if !ok || subnetsRaw == nil {
			continue
		}

		subnets := interfaceToStringsSlice(subnetsRaw)
		if len(subnets) == 0 {
			continue
		}

		if _, has := ic.Spec["mainNetwork"]; !has {
			ic.Spec["mainNetwork"] = subnets[0]
		}

		additionalNetworks := interfaceToStringsSlice(ic.Spec["additionalNetworks"])
		additionalNetworks = append(additionalNetworks, subnets[1:]...)
		ic.Spec["additionalNetworks"] = removeDuplicatesWithOrder(additionalNetworks)

		delete(ic.Spec, "subnets")

		patch := map[string]interface{}{"spec": map[string]interface{}{
			"mainNetwork":        ic.Spec["mainNetwork"],
			"additionalNetworks": ic.Spec["additionalNetworks"],
			"subnets":            nil,
		}}

		input.Logger.Info("migrating HuaweiCloudInstanceClass",
			slog.String("instanceClass", ic.Name),
		)

		input.PatchCollector.PatchWithMerge(
			patch,
			huaweiInstanceClassAPIVersion,
			"HuaweiCloudInstanceClass",
			"",
			ic.Name,
		)
	}
	return nil
}

func interfaceToStringsSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	res := make([]string, 0, len(arr))
	for _, val := range arr {
		if str, ok := val.(string); ok {
			res = append(res, str)
		}
	}
	return res
}

func removeDuplicatesWithOrder(subnets []string) []string {
	seen := make(map[string]struct{})
	for _, subnet := range subnets {
		seen[subnet] = struct{}{}
	}
	res := make([]string, 0, len(seen))
	for _, subnet := range subnets {
		if _, ok := seen[subnet]; ok {
			continue
		}
		seen[subnet] = struct{}{}
		res = append(res, subnet)
	}
	return res
}
