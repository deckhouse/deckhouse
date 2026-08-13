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
	"bytes"
	"encoding/json"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/capacity"
)

var specPassthroughKeys = []string{
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

type ResolveInput struct {
	Name              string
	ManualRolloutID   string
	NodeType          v1.NodeType
	Spec              v1.NodeGroupSpec
	Static            map[string]interface{}
	CloudProcessed    bool
	CloudProviderType string
}

// ResolvedNodeGroup is the NodeGroup with everything node-controller resolves on top of its
// spec: the effective Kubernetes version, the CRI type, the zones, the engine and the instance
// class. It is what the bashible context and the provider machine-class templates are built
// from, and ToMap is the single place it becomes the map those two consumers eat.
type ResolvedNodeGroup struct {
	Name              string
	NodeType          v1.NodeType
	Engine            string
	ManualRolloutID   string
	KubernetesVersion string
	CRIType           string
	SerializedLabels  string
	SerializedTaints  string
	UpdateEpoch       string

	// Spec is the allowlisted (specPassthroughKeys) passthrough of the NodeGroup spec. It stays a
	// map because these subtrees travel verbatim to the node bundles and the provider templates
	// and node-controller never reads them — describing them again here would be a second copy of
	// a schema that already exists, one that silently drops whatever the first one gains.
	Spec map[string]interface{}

	// Static is the static cluster configuration, carried by Static NodeGroups only.
	Static map[string]interface{}

	// CloudProviderType names the provider this NodeGroup resolved to, empty for a group outside
	// any cloud. It is deliberately outside the CloudProcessed gate below: bashible picks the
	// provider's step directory by it, and CloudPermanent never passes the cloud checks — gating it
	// would strip the provider steps from the master NodeGroup.
	CloudProviderType string

	// CloudProcessed reports that the cloud checks passed. It gates the whole cloud overlay,
	// including instanceClass, which is published even when it resolved to nil.
	CloudProcessed bool
	Zones          []string
	// InstanceClass is the provider's InstanceClass spec. The container is typed, the contents are
	// not: the schema belongs to a cloud-provider module and unknown keys must survive verbatim.
	InstanceClass map[string]any
	NodeCapacity  *capacity.InstanceType
}

func ResolveNodeGroup(in ResolveInput, r Result) ResolvedNodeGroup {
	resolved := ResolvedNodeGroup{
		Name:              in.Name,
		NodeType:          in.NodeType,
		Engine:            r.Engine,
		ManualRolloutID:   in.ManualRolloutID,
		KubernetesVersion: r.KubernetesVersion,
		CRIType:           r.CRIType,
		SerializedLabels:  r.SerializedLabels,
		SerializedTaints:  r.SerializedTaints,
		UpdateEpoch:       r.UpdateEpoch,
		Spec:              specPassthrough(in.Spec),
		CloudProviderType: in.CloudProviderType,
		CloudProcessed:    in.CloudProcessed,
	}

	if in.NodeType == v1.NodeTypeStatic {
		resolved.Static = in.Static
	}

	if in.CloudProcessed {
		resolved.Zones = r.Zones
		resolved.InstanceClass = r.InstanceClass
		resolved.NodeCapacity = r.NodeCapacity
	}

	return resolved
}

// ToMap serializes the resolved NodeGroup. Which keys it emits is data, not formatting:
// bashible-apiserver hashes the parsed context into every node's configuration checksum, so a
// key published empty must not become an absent key, and an absent one must not appear.
func (r ResolvedNodeGroup) ToMap() map[string]interface{} {
	out := make(map[string]interface{}, len(r.Spec)+12)
	for key, val := range r.Spec {
		out[key] = val
	}

	out["nodeType"] = string(r.NodeType)

	out["name"] = r.Name
	out["manualRolloutID"] = r.ManualRolloutID
	out["engine"] = r.Engine

	if len(r.Static) > 0 {
		out["static"] = r.Static
	}

	// Emitted only when the group has a provider: an always-present key would add "" to the
	// element of every Static NodeGroup and shift its configuration checksum for nothing.
	if r.CloudProviderType != "" {
		out["cloudProviderType"] = r.CloudProviderType
	}

	if r.CloudProcessed {
		// Published as the map its JSON form decodes to, not as the struct: the element also feeds
		// text/template, where a struct and a map render differently, and that rendering names an
		// immutable machine template.
		if r.NodeCapacity != nil {
			out["nodeCapacity"] = normalizeJSONMap(r.NodeCapacity)
		}
		// An unresolved InstanceClass is published as a plain null, not as a typed nil map: the
		// key must be present either way, and a typed nil is a different value to every consumer
		// that inspects the element rather than serializing it.
		if r.InstanceClass != nil {
			out["instanceClass"] = r.InstanceClass
		} else {
			out["instanceClass"] = nil
		}
		cloudInstances := copyMap(r.Spec["cloudInstances"])
		cloudInstances["zones"] = r.Zones
		out["cloudInstances"] = cloudInstances
	}

	out["kubernetesVersion"] = r.KubernetesVersion
	out["serializedLabels"] = r.SerializedLabels
	out["serializedTaints"] = r.SerializedTaints

	cri := copyMap(r.Spec["cri"])
	cri["type"] = r.CRIType
	out["cri"] = cri

	out["updateEpoch"] = r.UpdateEpoch

	return out
}

// specPassthrough returns the allowlisted subtrees of the spec. It goes through the spec's own
// JSON form rather than reading the typed fields one by one: the subtrees travel verbatim, and
// re-describing them here would be a second copy of a schema that already exists.
//
// The decode keeps integers as int64, which is what the API server's unstructured decode produces.
// A plain Unmarshal into interface{} widens them to float64; the two render the same today, but
// float64 loses precision above 2^53 and the result is hashed into every node's checksum.
func specPassthrough(spec v1.NodeGroupSpec) map[string]interface{} {
	raw, err := json.Marshal(spec)
	if err != nil {
		return map[string]interface{}{}
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var all map[string]interface{}
	if err := decoder.Decode(&all); err != nil {
		return map[string]interface{}{}
	}

	out := make(map[string]interface{}, len(specPassthroughKeys))
	for _, key := range specPassthroughKeys {
		val, ok := all[key]
		if !ok {
			continue
		}
		val = narrowNumbers(val)
		if isEmptySpecValue(val) {
			continue
		}
		out[key] = val
	}
	return out
}

// narrowNumbers turns json.Number into the concrete types unstructured data carries: int64 for
// integers, float64 for the rest.
func narrowNumbers(v interface{}) interface{} {
	switch val := v.(type) {
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return val.String()
	case map[string]interface{}:
		for k, inner := range val {
			val[k] = narrowNumbers(inner)
		}
		return val
	case []interface{}:
		for i, inner := range val {
			val[i] = narrowNumbers(inner)
		}
		return val
	default:
		return v
	}
}

func isEmptySpecValue(v interface{}) bool {
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
