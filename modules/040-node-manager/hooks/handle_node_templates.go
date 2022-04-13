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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ngv1 "github.com/deckhouse/deckhouse/go_lib/api/v1"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/go_lib/taints"
)

/**
HandleNodeTemplates hook applies annotations, taints and labels to CloudStatic, CloudPermanent and Static nodes
and deletes "node.deckhouse.io/uninitialized" taint.
*/

const (
	NodeGroupNameLabel                = "node.deckhouse.io/group"
	LastAppliedNodeTemplateAnnotation = "node-manager.deckhouse.io/last-applied-node-template"
	NodeUnininitalizedTaintKey        = "node.deckhouse.io/uninitialized"
)

type NodeSettings struct {
	Name                    string
	NodeType                ngv1.NodeType
	NodeGroup               string
	Annotations             map[string]string
	Labels                  map[string]string
	Taints                  []v1.Taint
	LastAppliedNodeTemplate *NodeSettings
}

// Hook will be executed when NodeType or NodeTemplate are changed.
func desiredNodeSettingsFromNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeGroup := new(ngv1.NodeGroup)
	err := sdk.FromUnstructured(obj, nodeGroup)
	if err != nil {
		return nil, err
	}

	settings := NodeSettings{
		Name:        nodeGroup.Name,
		NodeType:    nodeGroup.Spec.NodeType,
		Annotations: nodeGroup.Spec.NodeTemplate.Annotations,
		Labels:      nodeGroup.Spec.NodeTemplate.Labels,
		Taints:      nodeGroup.Spec.NodeTemplate.Taints,
	}

	// base64 decoding is not needed in Go.
	return settings, nil
}

// actualNodeSettingsFilter gets annotations, labels, taints, and nodeGroup
// from the Node. nodeGroup is used as a key to find desired settings.
// Hook will be executed when NodeGroup, annotations, labels or taints are changed.
func actualNodeSettingsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeObj := new(v1.Node)
	err := sdk.FromUnstructured(obj, nodeObj)
	if err != nil {
		return nil, err
	}

	settings := NodeSettings{
		Name:        nodeObj.Name,
		NodeGroup:   nodeObj.Labels[NodeGroupNameLabel],
		Labels:      nodeObj.Labels,
		Annotations: nodeObj.Annotations,
		Taints:      nodeObj.Spec.Taints,
	}

	lastApplied, ok := settings.Annotations[LastAppliedNodeTemplateAnnotation]
	if ok {
		settings.LastAppliedNodeTemplate = new(NodeSettings)
		err = json.Unmarshal([]byte(lastApplied), settings.LastAppliedNodeTemplate)
		if err != nil {
			return nil, fmt.Errorf("parse last applied node template: %v", err)
		}
	}

	return settings, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/handle_node_templates",
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "ngs",
			ApiVersion:             "deckhouse.io/v1",
			Kind:                   "NodeGroup",
			FilterFunc:             desiredNodeSettingsFromNodeGroupFilter,
			WaitForSynchronization: go_hook.Bool(false),
		},
		{
			Name:                   "nodes",
			ApiVersion:             "v1",
			Kind:                   "Node",
			FilterFunc:             actualNodeSettingsFilter,
			WaitForSynchronization: go_hook.Bool(false),
		},
	},
}, nodeTemplatesHandler)

// nodeTemplatesHandler applies annotations, labels and taints to Hybrid and Static nodes from NodeGroup's nodeTemplate.
// Also, "node.deckhouse.io/uninitialized" taint is deleted.
func nodeTemplatesHandler(input *go_hook.HookInput) error {
	nodes := input.Snapshots["nodes"]
	// Expire d8_unmanaged_nodes_on_cluster metric and register unmanaged nodes.
	// This is a separate loop because template applying may return an error.
	input.MetricsCollector.Expire("")
	for _, nodeObj := range nodes {
		node := nodeObj.(NodeSettings)
		if node.NodeGroup == "" {
			input.MetricsCollector.Set("d8_unmanaged_nodes_on_cluster", 1, map[string]string{
				"node": node.Name,
			})
		}
	}
	if len(nodes) == 0 {
		return nil
	}

	nodeGroups := input.Snapshots["ngs"]
	if len(nodeGroups) == 0 {
		return nil
	}

	// Prepare index of node groups.
	ngs := map[string]NodeSettings{}
	for _, nodeGroup := range nodeGroups {
		ng := nodeGroup.(NodeSettings)
		ngs[ng.Name] = ng
	}

	for _, nodeObj := range nodes {
		node := nodeObj.(NodeSettings)
		// Skip nodes not managed by us (not having node.deckhouse.io/group label)
		if node.NodeGroup == "" {
			continue
		}

		// Skip nodes from unknown node groups
		nodeGroup, ok := ngs[node.NodeGroup]
		if !ok {
			continue
		}

		input.PatchCollector.Filter(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			nodeObj := new(v1.Node)
			err := sdk.FromUnstructured(obj, nodeObj)
			if err != nil {
				return nil, err
			}

			if nodeGroup.NodeType == ngv1.NodeTypeCloudEphemeral {
				err = fixCloudNodeTaints(nodeObj, node, nodeGroup)
			} else {
				err = applyNodeTemplate(nodeObj, node, nodeGroup)
			}

			if err != nil {
				return nil, err
			}
			nodeObj.Status = v1.NodeStatus{}
			return sdk.ToUnstructured(nodeObj)
		}, "v1", "Node", "", node.Name)
	}

	return nil
}

// fixCloudNodeTaints removes "node.deckhouse.io/uninitialized" taint when
// NodeTemplate for Cloud node is applied by MCM.
// TODO Taint deletion should be moved to a separate hook in the future.
func fixCloudNodeTaints(nodeObj *v1.Node, node NodeSettings, nodeGroup NodeSettings) error {
	newTaints := taints.Slice(nodeObj.Spec.Taints).Merge(nodeGroup.Taints)

	if !newTaints.Equal(nodeObj.Spec.Taints) {
		return nil
	}
	newTaints = newTaints.WithoutKey(NodeUnininitalizedTaintKey)

	if len(newTaints) == 0 {
		// MergePatch: delete .spec.taints
		nodeObj.Spec.Taints = nil
	} else {
		// MergePatch: delete "node.deckhouse.io/uninitialized"
		nodeObj.Spec.Taints = newTaints
	}
	return nil
}

func applyNodeTemplate(nodeObj *v1.Node, node NodeSettings, nodeGroup NodeSettings) error {
	// 1. Labels
	// 1.1. Merge node.labels with nodeTemplate.labels and remove excess keys.
	var lastLabels map[string]string
	if node.LastAppliedNodeTemplate != nil {
		lastLabels = node.LastAppliedNodeTemplate.Labels
	}
	newLabels, labelsChanged := ApplyTemplateMap(node.Labels, nodeGroup.Labels, lastLabels)

	// 1.2. Add label with nodeGroup name.
	_, ok := newLabels["node-role.kubernetes.io/"+nodeGroup.Name]
	if !ok {
		labelsChanged = true
	}
	newLabels["node-role.kubernetes.io/"+nodeGroup.Name] = ""

	// 1.3. Add label with nodeGroup type
	nodeGroupType, ok := newLabels["node.deckhouse.io/type"]
	if !ok || nodeGroupType != nodeGroup.NodeType.String() {
		labelsChanged = true
	}
	newLabels["node.deckhouse.io/type"] = nodeGroup.NodeType.String()

	// 2. Annotations
	// 2.1. Merge node.annotations with nodeTemplate.annotations and remove excess keys.
	var lastAnnotations map[string]string
	if node.LastAppliedNodeTemplate != nil {
		lastAnnotations = node.LastAppliedNodeTemplate.Annotations
	}
	newAnnotations, annotationsChanged := ApplyTemplateMap(node.Annotations, nodeGroup.Annotations, lastAnnotations)

	// 2.2. Save last applied node template in annotation.
	// Mimic shell-operator behaviour with empty fields.
	lastAppliedMap := map[string]interface{}{
		"annotations": make(map[string]string),
		"labels":      make(map[string]string),
		"taints":      make([]v1.Taint, 0),
	}
	if len(nodeGroup.Annotations) > 0 {
		lastAppliedMap["annotations"] = nodeGroup.Annotations
	}
	if len(nodeGroup.Labels) > 0 {
		lastAppliedMap["labels"] = nodeGroup.Labels
	}
	if len(nodeGroup.Taints) > 0 {
		lastAppliedMap["taints"] = nodeGroup.Taints
	}
	newLastApplied, err := json.Marshal(lastAppliedMap)
	if err != nil {
		return fmt.Errorf("marshal last-applied-node-template: %v", err)
	}

	value, ok := newAnnotations[LastAppliedNodeTemplateAnnotation]
	if !ok || value != string(newLastApplied) {
		annotationsChanged = true
	}
	newAnnotations[LastAppliedNodeTemplateAnnotation] = string(newLastApplied)

	// 3. Taints
	// 3.1. Merge taints, remove excess.
	var lastTaints []v1.Taint
	if node.LastAppliedNodeTemplate != nil {
		lastTaints = node.LastAppliedNodeTemplate.Taints
	}
	newTaints, taintsChanged := taints.Slice(node.Taints).ApplyTemplate(nodeGroup.Taints, lastTaints)

	// 3.2. Delete uninitialized taint.
	if newTaints.HasKey(NodeUnininitalizedTaintKey) {
		taintsChanged = true
		newTaints = newTaints.WithoutKey(NodeUnininitalizedTaintKey)
	}

	if labelsChanged || annotationsChanged {
		if labelsChanged {
			nodeObj.SetLabels(newLabels)
		}
		if annotationsChanged {
			nodeObj.SetAnnotations(newAnnotations)
		}
	}

	if taintsChanged {
		if len(newTaints) == 0 {
			// 3.3. Remove taint key if no taints left.
			nodeObj.Spec.Taints = nil
		} else {
			nodeObj.Spec.Taints = newTaints
		}
	}

	return nil
}

// ApplyTemplateMap return actual merged with template without excess keys.
func ApplyTemplateMap(actual, template, lastApplied map[string]string) (map[string]string, bool) {
	changed := false
	excess := ExcessMapKeys(lastApplied, template)
	newMap := map[string]string{}

	for k, v := range actual {
		// Ignore keys removed from template.
		if excess.Has(k) {
			changed = true
			continue
		}
		newMap[k] = v
	}

	// Merge with values from template.
	for k, v := range template {
		oldVal, ok := newMap[k]
		if !ok || oldVal != v {
			changed = true
		}
		newMap[k] = v
	}

	return newMap, changed
}

// ExcessMapKeys returns keys from a without keys from b.
func ExcessMapKeys(a, b map[string]string) set.Set {
	onlyA := set.New()
	for k := range a {
		onlyA.Add(k)
	}
	for k := range b {
		onlyA.Delete(k)
	}
	return onlyA
}
