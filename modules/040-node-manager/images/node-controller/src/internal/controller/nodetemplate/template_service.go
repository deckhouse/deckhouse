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

func applyNodeTemplate(nodeObj *corev1.Node, nodeGroup *v1.NodeGroup) error {
	var lastAppliedNodeTemplate *v1.NodeTemplate

	if nodeObj.Annotations != nil {
		if lastApplied := nodeObj.Annotations[lastAppliedNodeTemplateAnnotation]; lastApplied != "" {
			lant := v1.NodeTemplate{}
			if err := json.Unmarshal([]byte(lastApplied), &lant); err != nil {
				return fmt.Errorf("parse last applied node template: %w", err)
			}
			lastAppliedNodeTemplate = &lant
		}
	}

	actualLabels := cloneStringMap(nodeObj.Labels)
	delete(actualLabels, metalLBmemberLabelKey)
	desiredLabels := getTemplateLabels(nodeGroup)
	var lastLabels map[string]string
	if lastAppliedNodeTemplate != nil {
		lastLabels = lastAppliedNodeTemplate.Labels
	}
	newLabels, labelsChanged := applyTemplateMap(actualLabels, desiredLabels, lastLabels)

	roleLabel := "node-role.kubernetes.io/" + nodeGroup.Name
	if value, ok := newLabels[roleLabel]; !ok || value != "" {
		labelsChanged = true
	}
	newLabels[roleLabel] = ""

	nodeType := string(nodeGroup.Spec.NodeType)
	if value, ok := newLabels["node.deckhouse.io/type"]; !ok || value != nodeType {
		labelsChanged = true
	}
	newLabels["node.deckhouse.io/type"] = nodeType

	actualAnnotations := cloneStringMap(nodeObj.Annotations)
	delete(actualAnnotations, heartbeatAnnotationKey)
	desiredAnnotations := getTemplateAnnotations(nodeGroup)
	var lastAnnotations map[string]string
	if lastAppliedNodeTemplate != nil {
		lastAnnotations = lastAppliedNodeTemplate.Annotations
	}
	newAnnotations, annotationsChanged := applyTemplateMap(actualAnnotations, desiredAnnotations, lastAnnotations)

	lastAppliedMap := map[string]interface{}{
		"annotations": map[string]string{},
		"labels":      map[string]string{},
		"taints":      make([]corev1.Taint, 0),
	}
	if len(desiredAnnotations) > 0 {
		lastAppliedMap["annotations"] = desiredAnnotations
	}
	if len(desiredLabels) > 0 {
		lastAppliedMap["labels"] = desiredLabels
	}
	templateTaints := getTemplateTaints(nodeGroup)
	if len(templateTaints) > 0 {
		lastAppliedMap["taints"] = templateTaints
	}

	newLastApplied, err := json.Marshal(lastAppliedMap)
	if err != nil {
		return fmt.Errorf("marshal last applied node template: %w", err)
	}
	if value, ok := newAnnotations[lastAppliedNodeTemplateAnnotation]; !ok || value != string(newLastApplied) {
		annotationsChanged = true
	}
	newAnnotations[lastAppliedNodeTemplateAnnotation] = string(newLastApplied)

	var lastTaints []corev1.Taint
	if lastAppliedNodeTemplate != nil {
		lastTaints = lastAppliedNodeTemplate.Taints
	}
	newTaints, taintsChanged := applyTemplateTaints(nodeObj.Spec.Taints, templateTaints, lastTaints)
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
		if len(newTaints) == 0 {
			nodeObj.Spec.Taints = nil
		} else {
			nodeObj.Spec.Taints = newTaints
		}
	}

	return nil
}

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
