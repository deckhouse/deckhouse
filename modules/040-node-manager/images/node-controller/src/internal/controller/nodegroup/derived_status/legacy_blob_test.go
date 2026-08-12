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
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// This file is a frozen copy of BuildNodeGroupBlob and its helpers as they were before the
// element was typed. It is the reference the production builder is diffed against, so nothing
// here may be refactored along with the production code — including the helpers, which is why
// they are duplicated instead of called.

var legacyNodeGroupForValuesKeys = []string{
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

func legacyBuildNodeGroupBlob(in ResolveInput, r Result) map[string]interface{} {
	blob := make(map[string]interface{})

	for _, key := range legacyNodeGroupForValuesKeys {
		val, ok := in.RawSpec[key]
		if !ok {
			continue
		}
		if legacyIsEmptyBlobValue(val) {
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
		if capacity := legacyRawExtensionToValue(r.NodeCapacity); capacity != nil {
			blob["nodeCapacity"] = capacity
		}
		blob["instanceClass"] = legacyRawExtensionToValue(r.InstanceClass)
		cloudInstances := legacyCopyMap(blob["cloudInstances"])
		cloudInstances["zones"] = r.Zones
		blob["cloudInstances"] = cloudInstances
	}

	blob["kubernetesVersion"] = r.KubernetesVersion
	blob["serializedLabels"] = r.SerializedLabels
	blob["serializedTaints"] = r.SerializedTaints

	cri := legacyCopyMap(blob["cri"])
	cri["type"] = r.CRIType
	blob["cri"] = cri

	blob["updateEpoch"] = r.UpdateEpoch

	return blob
}

func legacyIsEmptyBlobValue(v interface{}) bool {
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

func legacyCopyMap(v interface{}) map[string]interface{} {
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

// legacyRawExtensionToValue is the frozen conversion the legacy builder applied to the two cloud
// fields. Those fields are typed now, so the adapter takes the value directly and round-trips it
// through JSON — which is exactly what the RawExtension version did, byte for byte. The frozen
// algorithm above is untouched; only the plumbing at its edge follows the field types.
func legacyRawExtensionToValue(value interface{}) interface{} {
	if value == nil || reflect.ValueOf(value).IsZero() {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// TestResolvedNodeGroup_MatchesFrozenLegacy compares the live values, not their serialization:
// the element also feeds text/template, where []string and []interface{} render differently and
// the rendered instance-class checksum names an immutable machine template.
func TestResolvedNodeGroup_MatchesFrozenLegacy(t *testing.T) {
	for _, fixture := range nodeGroupCorpus() {
		t.Run(fixture.name, func(t *testing.T) {
			require.Equal(t,
				legacyBuildNodeGroupBlob(fixture.input, fixture.result),
				resolvedMap(fixture.input, fixture.result),
			)
		})
	}
}
