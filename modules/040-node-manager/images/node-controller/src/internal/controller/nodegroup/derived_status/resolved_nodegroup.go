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
	Name            string
	ManualRolloutID string
	NodeType        v1.NodeType
	RawSpec         map[string]interface{}
	Static          map[string]interface{}
	CloudProcessed  bool
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

	// Spec is the allowlisted (specPassthroughKeys) passthrough of the NodeGroup spec, kept
	// as the raw unstructured values. These subtrees are read by the node bundles and by the
	// provider templates, never by node-controller, so typing them here would only add a
	// conversion that can lose data.
	Spec map[string]interface{}

	// Static is the static cluster configuration, carried by Static NodeGroups only.
	Static map[string]interface{}

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
		Spec:              specPassthrough(in.RawSpec),
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

	if r.CloudProcessed {
		// Published as the map its JSON form decodes to, not as the struct: the element also feeds
		// text/template, where a struct and a map render differently, and that rendering names an
		// immutable machine template.
		if capacityValue := normalizeJSONMap(r.NodeCapacity); capacityValue != nil {
			out["nodeCapacity"] = capacityValue
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

func specPassthrough(rawSpec map[string]interface{}) map[string]interface{} {
	spec := make(map[string]interface{}, len(specPassthroughKeys))
	for _, key := range specPassthroughKeys {
		val, ok := rawSpec[key]
		if !ok {
			continue
		}
		if isEmptySpecValue(val) {
			continue
		}
		spec[key] = val
	}
	return spec
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
