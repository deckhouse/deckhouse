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

// nodeGroupForValuesKeys are the spec keys that get_crds.nodeGroupForValues
// (get_crds.go:580-618) copies through verbatim into the internal.nodeGroups
// blob element. The order does not matter (the blob is re-marshaled through
// sigs.k8s.io/yaml with sorted keys before checksum), only the key set does.
//
// spec.update is intentionally absent: nodeGroupForValues does NOT pass it
// through, so the blob must not contain it either.
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

// BlobInput carries the per-NodeGroup inputs for blob assembly. RawSpec must be
// the NodeGroup's .spec as stored by the apiserver (CRD-shaped, unknown fields
// already pruned). Building the blob from this raw map — rather than from the
// hand-rolled node-controller v1.NodeGroupSpec, whose json shape diverges from
// the CRD (e.g. gpu.mode vs gpu.sharing) — is what preserves byte-parity with
// get_crds and prevents mass node re-bootstrap.
type BlobInput struct {
	Name            string
	ManualRolloutID string
	NodeType        v1.NodeType
	RawSpec         map[string]interface{}
	// Static is the internal.static value (decoded d8-static-cluster-configuration),
	// embedded only for Static NodeGroups, matching get_crds.go:345-351.
	Static map[string]interface{}
	// CloudProcessed is true for a CloudEphemeral NodeGroup that passed the
	// provider/instance-class checks, i.e. the branch where get_crds sets
	// instanceClass, nodeCapacity and cloudInstances.zones (get_crds.go:353-477).
	CloudProcessed bool
}

// BuildNodeGroupBlob assembles the internal.nodeGroups blob element for a single
// NodeGroup, mirroring the get_crds hook loop (get_crds.go:332-554). Passthrough
// spec keys are copied verbatim from RawSpec; the computed fields come from r.
func BuildNodeGroupBlob(in BlobInput, r Result) map[string]interface{} {
	blob := make(map[string]interface{})

	// Spec passthrough (whitelisted keys only). nodeType is always present;
	// the rest are copied only when non-empty, matching the IsEmpty guards in
	// nodeGroupForValues.
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
		// nodeCapacity is added only for scale-from-zero (get_crds.go:416-429);
		// r.NodeCapacity is set only in that case.
		if capacity := rawExtensionToValue(r.NodeCapacity); capacity != nil {
			blob["nodeCapacity"] = capacity
		}
		// instanceClass is always present for a processed cloud NG, even when the
		// resolved spec is empty (get_crds emits "instanceClass": null, see the
		// get_crds_test.go golden fixtures).
		blob["instanceClass"] = rawExtensionToValue(r.InstanceClass)
		cloudInstances := copyMap(blob["cloudInstances"])
		cloudInstances["zones"] = r.Zones
		blob["cloudInstances"] = cloudInstances
	}

	blob["kubernetesVersion"] = r.KubernetesVersion
	blob["serializedLabels"] = r.SerializedLabels
	blob["serializedTaints"] = r.SerializedTaints

	// cri is always present after the resolved type is applied, even when the
	// spec carried no cri block (get_crds.go:524-529).
	cri := copyMap(blob["cri"])
	cri["type"] = r.CRIType
	blob["cri"] = cri

	blob["updateEpoch"] = r.UpdateEpoch

	return blob
}

// isEmptyBlobValue reports whether a passthrough spec value should be dropped,
// approximating the ngv1 IsEmpty guards for CRD-shaped raw values.
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

// copyMap returns a shallow copy of v as a map, or a fresh map when v is not a
// map. It avoids mutating the source RawSpec when overlaying cri.type / zones.
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

// rawExtensionToValue decodes a RawExtension's JSON into a generic value so it
// embeds as a nested structure in the blob (not as an opaque string).
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
