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
	"cmp"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// fixMasterTaints removes the legacy node-role.kubernetes.io/master taint from a master node when the
// NodeGroup template does not declare it and the node carries no control-plane taint yet.
func fixMasterTaints(nodeTaints, ngTaints []corev1.Taint) []corev1.Taint {
	if len(nodeTaints) == 0 {
		return nodeTaints
	}

	ngTaintsMap := make(map[string]struct{}, len(ngTaints))
	for _, ngTaint := range ngTaints {
		ngTaintsMap[ngTaint.Key] = struct{}{}
	}

	nodeTaintsMap := make(map[string]corev1.Taint, len(nodeTaints))
	for _, sourceTaint := range nodeTaints {
		nodeTaintsMap[sourceTaint.Key] = sourceTaint
	}

	if _, ok := nodeTaintsMap[controlPlaneTaintKey]; !ok {
		_, existsInNG := ngTaintsMap[masterNodeRoleKey]
		_, existsInNodeSpec := nodeTaintsMap[masterNodeRoleKey]
		if existsInNodeSpec && !existsInNG {
			delete(nodeTaintsMap, masterNodeRoleKey)
			newTaints := make([]corev1.Taint, 0, len(nodeTaintsMap))
			for _, v := range nodeTaintsMap {
				newTaints = append(newTaints, v)
			}
			return newTaints
		}
	}

	return nodeTaints
}

// fixCloudNodeTaints clears the uninitialized taint on a CloudEphemeral node once its taints already
// include the template taints, which the machine controller sets. Until then the node is left alone.
func fixCloudNodeTaints(nodeObj *corev1.Node, nodeGroup *v1.NodeGroup) {
	newTaints := mergeTaints(nodeObj.Spec.Taints, getTemplateTaints(nodeGroup))
	if !taintSliceEqual(newTaints, nodeObj.Spec.Taints) {
		return
	}
	newTaints = taintSliceWithoutKey(newTaints, nodeUninitializedTaintKey)

	if len(newTaints) == 0 {
		nodeObj.Spec.Taints = nil
	} else {
		nodeObj.Spec.Taints = newTaints
	}
}

func taintSliceHasKey(ts []corev1.Taint, key string) bool {
	for _, t := range ts {
		if t.Key == key {
			return true
		}
	}
	return false
}

func taintSliceWithoutKey(ts []corev1.Taint, key string) []corev1.Taint {
	result := make([]corev1.Taint, 0, len(ts))
	for _, t := range ts {
		if t.Key != key {
			result = append(result, t)
		}
	}
	return result
}

func taintID(t corev1.Taint) string {
	return t.Key + "|" + string(t.Effect)
}

func taintSliceEqual(a, b []corev1.Taint) bool {
	if len(a) != len(b) {
		return false
	}
	mapA := make(map[string]corev1.Taint, len(a))
	for _, t := range a {
		mapA[taintID(t)] = t
	}
	for _, t := range b {
		v, ok := mapA[taintID(t)]
		if !ok {
			return false
		}
		if v.Value != t.Value || v.Key != t.Key || v.Effect != t.Effect {
			return false
		}
	}
	return true
}

func mergeTaints(actual, template []corev1.Taint) []corev1.Taint {
	out := append([]corev1.Taint(nil), actual...)
	index := make(map[string]int, len(out))
	for i := range out {
		index[taintID(out[i])] = i
	}
	for _, t := range template {
		id := taintID(t)
		if i, ok := index[id]; ok {
			out[i] = t
			continue
		}
		out = append(out, t)
		index[id] = len(out) - 1
	}
	return out
}

func taintToString(t corev1.Taint) string {
	return t.Key + "=" + t.Value + ":" + string(t.Effect)
}

// ownedTaints lists the taints node-controller may remove from a node: the ones it applied from the
// template before, or on the first reconcile of a master the control-plane taint cluster bootstrap puts
// there (candi/bashible/common-steps/cluster-bootstrap/072_install_control_plane.sh.tpl).
func ownedTaints(lastApplied *v1.NodeTemplate, nodeGroup *v1.NodeGroup) []corev1.Taint {
	if lastApplied != nil {
		return lastApplied.Taints
	}
	if nodeGroup.Name == masterNodeGroupName {
		return []corev1.Taint{{Key: controlPlaneTaintKey, Effect: corev1.TaintEffectNoSchedule}}
	}
	return nil
}

// applyTemplateTaints merges template taints into the node taints, one taint per key. An owned taint
// that left the template is dropped. Every other taint on the node (CCM, CSI, bashible, set by hand)
// is kept as is.
func applyTemplateTaints(actual, template, owned []corev1.Taint) ([]corev1.Taint, bool) {
	changed := false
	byKey := make(map[string]corev1.Taint, len(actual)+len(template))
	for _, t := range actual {
		if taintSliceHasKey(owned, t.Key) && !taintSliceHasKey(template, t.Key) {
			changed = true
			continue
		}
		byKey[t.Key] = t
	}
	for _, t := range template {
		if old, ok := byKey[t.Key]; !ok || taintToString(old) != taintToString(t) {
			changed = true
		}
		byKey[t.Key] = t
	}
	sorted := slices.SortedFunc(maps.Values(byKey), func(a, b corev1.Taint) int { return cmp.Compare(a.Key, b.Key) })
	return sorted, changed
}
