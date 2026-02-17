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
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

// decodeDataFromSecret returns data section from Secret. If possible, top level keys are converted from JSON.
func decodeDataFromSecret(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{}
	for k, v := range secret.Data {
		res[k] = string(v)
		// Try to load JSON from value.
		var jsonValue interface{}
		err := json.Unmarshal(v, &jsonValue)
		if err == nil {
			switch v := jsonValue.(type) {
			case map[string]interface{}:
				res[k] = v
			case []interface{}:
				res[k] = v
			case string:
				res[k] = v
				// This default will convert numbers into float64. It seems not ok for secret data.
				//default:
				//	res[k] = jsonValue
			}
		}
	}

	return res, nil
}

// semverMajMin is a Go implementation of this bash snippet:
//
//	function semver::majmin() {
//	  echo "$(echo $1 | cut -d. -f1,2)"
//	}
func semverMajMin(ver *semver.Version) string {
	if ver == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d", ver.Major(), ver.Minor())
}

// semverMin is a function that finds the minimum semver in a slice.
func semverMin(versions []*semver.Version) *semver.Version {
	if len(versions) == 0 {
		return nil
	}
	var res *semver.Version
	for i, ver := range versions {
		if res == nil || res.GreaterThan(ver) {
			res = versions[i]
		}
	}
	return res
}

func patchNodeGroupStatus(patcher go_hook.PatchCollector, nodeGroupName string, patch interface{}) {
	patcher.PatchWithMerge(patch, "deckhouse.io/v1", "NodeGroup", "", nodeGroupName, object_patch.WithSubresource("/status"))
}

func setNodeGroupStatus(patcher go_hook.PatchCollector, nodeGroupName string, statusField string, value interface{}) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			statusField: value,
		},
	}
	patchNodeGroupStatus(patcher, nodeGroupName, statusPatch)
}
func conditionsToPatch(conditions []ngv1.NodeGroupCondition) []map[string]interface{} {
	res := make([]map[string]interface{}, 0, len(conditions))

	for _, cc := range conditions {
		res = append(res, cc.ToMap())
	}

	return res
}

const (
	minStatusField                 = "min"
	maxStatusField                 = "max"
	desiredStatusField             = "desired"
	instancesStatusField           = "instances"
	lastMachineFailuresStatusField = "lastMachineFailures"
)
