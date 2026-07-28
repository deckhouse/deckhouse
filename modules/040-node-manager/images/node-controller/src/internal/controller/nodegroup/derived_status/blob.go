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

// Element is what node-controller derives per NodeGroup: the bashible context entry and the
// render context of the provider machine-class templates. ToMap is the single place it becomes
// the map those two consumers eat.
type Element struct {
	Name              string
	NodeType          v1.NodeType
	Engine            string
	ManualRolloutID   string
	KubernetesVersion string
	CRIType           string
	SerializedLabels  string
	SerializedTaints  string
	UpdateEpoch       string

	// Spec is the allowlisted (nodeGroupForValuesKeys) passthrough of the NodeGroup spec, kept
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
	InstanceClass  interface{}
	NodeCapacity   interface{}
}

func BuildNodeGroupElement(in BlobInput, r Result) Element {
	element := Element{
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
		element.Static = in.Static
	}

	if in.CloudProcessed {
		element.Zones = r.Zones
		element.InstanceClass = rawExtensionToValue(r.InstanceClass)
		element.NodeCapacity = rawExtensionToValue(r.NodeCapacity)
	}

	return element
}

// ToMap serializes the element. Which keys it emits is data, not formatting: bashible-apiserver
// hashes the parsed context into every node's configuration checksum, so a key published empty
// must not become an absent key, and an absent one must not appear.
func (e Element) ToMap() map[string]interface{} {
	blob := make(map[string]interface{}, len(e.Spec)+12)
	for key, val := range e.Spec {
		blob[key] = val
	}

	blob["nodeType"] = string(e.NodeType)

	blob["name"] = e.Name
	blob["manualRolloutID"] = e.ManualRolloutID
	blob["engine"] = e.Engine

	if len(e.Static) > 0 {
		blob["static"] = e.Static
	}

	if e.CloudProcessed {
		if e.NodeCapacity != nil {
			blob["nodeCapacity"] = e.NodeCapacity
		}
		blob["instanceClass"] = e.InstanceClass
		cloudInstances := copyMap(e.Spec["cloudInstances"])
		cloudInstances["zones"] = e.Zones
		blob["cloudInstances"] = cloudInstances
	}

	blob["kubernetesVersion"] = e.KubernetesVersion
	blob["serializedLabels"] = e.SerializedLabels
	blob["serializedTaints"] = e.SerializedTaints

	cri := copyMap(e.Spec["cri"])
	cri["type"] = e.CRIType
	blob["cri"] = cri

	blob["updateEpoch"] = e.UpdateEpoch

	return blob
}

func specPassthrough(rawSpec map[string]interface{}) map[string]interface{} {
	spec := make(map[string]interface{}, len(nodeGroupForValuesKeys))
	for _, key := range nodeGroupForValuesKeys {
		val, ok := rawSpec[key]
		if !ok {
			continue
		}
		if isEmptyBlobValue(val) {
			continue
		}
		spec[key] = val
	}
	return spec
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
