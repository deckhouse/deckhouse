/*
Copyright 2026 Flant JSC

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

package derived_status

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var nodeGroupForValuesKeys = []string{
	"nodeType",
	"cri",
	"gpu",
	"staticInstances",
	"cloudInstances",
	"nodeTemplate",
	"chaos",
	"operatingSystem",
	"disruptions",
	"kubelet",
	"fencing",
	"nodeDrainTimeoutSecond",
}

type BlobInput struct {
	Name            string
	ManualRolloutID string
	NodeType        v1.NodeType
	RawSpec         map[string]interface{}
	Static          map[string]interface{}
	CloudProcessed  bool
}

func BuildNodeGroupBlob(in BlobInput, r Result) map[string]interface{} {
	blob := make(map[string]interface{})

	for _, key := range nodeGroupForValuesKeys {
		val, ok := in.RawSpec[key]
		if !ok {
			continue
		}
		if key != "nodeType" && isEmptyBlobValue(val) {
			continue
		}
		blob[key] = val
	}
	blob["nodeType"] = string(in.NodeType)

	blob["name"] = in.Name
	blob["manualRolloutID"] = in.ManualRolloutID
	blob["engine"] = r.Engine

	if in.NodeType == v1.NodeTypeStatic && len(in.Static) > 0 {
		blob["static"] = in.Static
	}

	if in.CloudProcessed {
		if capacity := rawExtensionToValue(r.NodeCapacity); capacity != nil {
			blob["nodeCapacity"] = capacity
		}
		blob["instanceClass"] = rawExtensionToValue(r.InstanceClass)
		cloudInstances := copyMap(blob["cloudInstances"])
		cloudInstances["zones"] = r.Zones
		blob["cloudInstances"] = cloudInstances
	}

	blob["kubernetesVersion"] = r.KubernetesVersion
	blob["serializedLabels"] = r.SerializedLabels
	blob["serializedTaints"] = r.SerializedTaints

	cri := copyMap(blob["cri"])
	cri["type"] = r.CRIType
	blob["cri"] = cri

	blob["updateEpoch"] = r.UpdateEpoch

	return blob
}

func isEmptyBlobValue(v interface{}) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return val == ""
	case map[string]interface{}:
		return len(val) == 0
	case []interface{}:
		return len(val) == 0
	default:
		return false
	}
}

func copyMap(v interface{}) map[string]interface{} {
	src, ok := v.(map[string]interface{})
	if !ok {
		return make(map[string]interface{})
	}
	dst := make(map[string]interface{}, len(src)+1)
	for k, val := range src {
		dst[k] = val
	}
	return dst
}

func rawExtensionToValue(ext *runtime.RawExtension) interface{} {
	if ext == nil || len(ext.Raw) == 0 {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(ext.Raw, &out); err != nil {
		return nil
	}
	return out
}
