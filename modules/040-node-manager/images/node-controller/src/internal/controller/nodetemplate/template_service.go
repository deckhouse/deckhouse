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

package nodetemplate

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// applyNodeTemplate brings node labels, annotations and taints in line with the NodeGroup template:
// template entries are set, entries a previous template declared and this one dropped are removed,
// everything else stays. nodeObj is changed in place, the caller patches it.
func applyNodeTemplate(nodeObj *corev1.Node, nodeGroup *v1.NodeGroup) error {
	lastApplied, err := parseLastAppliedNodeTemplate(nodeObj)
	if err != nil {
		return err
	}

	newLabels, labelsChanged := templateLabels(nodeObj, nodeGroup, lastApplied)
	newAnnotations, annotationsChanged, err := templateAnnotations(nodeObj, nodeGroup, lastApplied)
	if err != nil {
		return err
	}
	newTaints, taintsChanged := applyTemplateTaints(nodeObj.Spec.Taints, getTemplateTaints(nodeGroup), ownedTaints(lastApplied, nodeGroup))
	if taintSliceHasKey(newTaints, nodeUninitializedTaintKey) {
		taintsChanged = true
		newTaints = taintSliceWithoutKey(newTaints, nodeUninitializedTaintKey)
	}

	if labelsChanged {
		nodeObj.Labels = newLabels
	}
	if annotationsChanged {
		nodeObj.Annotations = newAnnotations
	}
	if taintsChanged {
		nodeObj.Spec.Taints = newTaints
		if len(newTaints) == 0 {
			nodeObj.Spec.Taints = nil
		}
	}
	return nil
}

// parseLastAppliedNodeTemplate reads the template applied by the previous reconcile from the node
// annotation. nil means the node has not been reconciled yet.
func parseLastAppliedNodeTemplate(nodeObj *corev1.Node) (*v1.NodeTemplate, error) {
	raw := nodeObj.Annotations[lastAppliedNodeTemplateAnnotation]
	if raw == "" {
		return nil, nil
	}
	var lastApplied v1.NodeTemplate
	if err := json.Unmarshal([]byte(raw), &lastApplied); err != nil {
		return nil, fmt.Errorf("parse last applied node template: %w", err)
	}
	return &lastApplied, nil
}

// templateLabels merges template labels into the node labels and sets the system labels: the role
// label named after the NodeGroup and the node type. The MetalLB member label is dropped from the copy.
func templateLabels(nodeObj *corev1.Node, nodeGroup *v1.NodeGroup, lastApplied *v1.NodeTemplate) (map[string]string, bool) {
	actual := cloneStringMap(nodeObj.Labels)
	delete(actual, metalLBmemberLabelKey)
	var last map[string]string
	if lastApplied != nil {
		last = lastApplied.Labels
	}
	labels, changed := applyTemplateMap(actual, getTemplateLabels(nodeGroup), last)

	systemLabels := map[string]string{
		nodeRoleLabelPrefix + nodeGroup.Name: "",
		nodeTypeLabel:                        string(nodeGroup.Spec.NodeType),
	}
	for key, value := range systemLabels {
		if old, ok := labels[key]; !ok || old != value {
			changed = true
		}
		labels[key] = value
	}
	return labels, changed
}

// templateAnnotations merges template annotations into the node annotations and records the template
// being applied in the last-applied annotation. The kubevirt heartbeat annotation is dropped from the copy.
func templateAnnotations(nodeObj *corev1.Node, nodeGroup *v1.NodeGroup, lastApplied *v1.NodeTemplate) (map[string]string, bool, error) {
	actual := cloneStringMap(nodeObj.Annotations)
	delete(actual, heartbeatAnnotationKey)
	var last map[string]string
	if lastApplied != nil {
		last = lastApplied.Annotations
	}
	annotations, changed := applyTemplateMap(actual, getTemplateAnnotations(nodeGroup), last)

	newLastApplied, err := marshalLastAppliedNodeTemplate(nodeGroup)
	if err != nil {
		return nil, false, err
	}
	if annotations[lastAppliedNodeTemplateAnnotation] != newLastApplied {
		changed = true
	}
	annotations[lastAppliedNodeTemplateAnnotation] = newLastApplied
	return annotations, changed, nil
}

// marshalLastAppliedNodeTemplate serializes the template for the last-applied annotation. All three
// keys are always present: the format is shared with the pre-2026 hook.
func marshalLastAppliedNodeTemplate(nodeGroup *v1.NodeGroup) (string, error) {
	lastApplied := map[string]any{
		"annotations": map[string]string{},
		"labels":      map[string]string{},
		"taints":      []corev1.Taint{},
	}
	if annotations := getTemplateAnnotations(nodeGroup); len(annotations) > 0 {
		lastApplied["annotations"] = annotations
	}
	if labels := getTemplateLabels(nodeGroup); len(labels) > 0 {
		lastApplied["labels"] = labels
	}
	if taints := getTemplateTaints(nodeGroup); len(taints) > 0 {
		lastApplied["taints"] = taints
	}
	raw, err := json.Marshal(lastApplied)
	if err != nil {
		return "", fmt.Errorf("marshal last applied node template: %w", err)
	}
	return string(raw), nil
}

// applyTemplateMap is the label and annotation counterpart of applyTemplateTaints: template entries
// are set, keys that were in lastApplied but left the template are removed, the rest is kept.
func applyTemplateMap(actual, template, lastApplied map[string]string) (map[string]string, bool) {
	changed := false
	excess := excessMapKeys(lastApplied, template)
	newMap := map[string]string{}

	for k, v := range actual {
		if _, found := excess[k]; found {
			changed = true
			continue
		}
		newMap[k] = v
	}

	for k, v := range template {
		oldVal, ok := newMap[k]
		if !ok || oldVal != v {
			changed = true
		}
		newMap[k] = v
	}

	return newMap, changed
}

// excessMapKeys returns the keys present in a but not in b.
func excessMapKeys(a, b map[string]string) map[string]struct{} {
	onlyA := make(map[string]struct{}, len(a))
	for k := range a {
		onlyA[k] = struct{}{}
	}
	for k := range b {
		delete(onlyA, k)
	}
	return onlyA
}
