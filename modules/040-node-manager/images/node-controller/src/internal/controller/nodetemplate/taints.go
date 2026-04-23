package nodetemplate

import (
	"slices"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

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

func applyTemplateTaints(actual, template, lastApplied []corev1.Taint) ([]corev1.Taint, bool) {
	changed := false
	templateSet := make(map[string]struct{}, len(template))
	for _, t := range template {
		templateSet[taintID(t)] = struct{}{}
	}

	removeSet := make(map[string]struct{})
	for _, t := range lastApplied {
		id := taintID(t)
		if _, found := templateSet[id]; !found {
			removeSet[id] = struct{}{}
		}
	}

	result := make([]corev1.Taint, 0, len(actual)+len(template))
	index := make(map[string]int, len(actual)+len(template))

	for _, t := range actual {
		id := taintID(t)
		if _, shouldRemove := removeSet[id]; shouldRemove {
			changed = true
			continue
		}
		result = append(result, t)
		index[id] = len(result) - 1
	}

	for _, t := range template {
		id := taintID(t)
		if i, ok := index[id]; ok {
			if result[i].Value != t.Value {
				changed = true
			}
			result[i] = t
			continue
		}
		changed = true
		result = append(result, t)
		index[id] = len(result) - 1
	}

	slices.SortFunc(result, func(a, b corev1.Taint) int {
		aid := taintID(a)
		bid := taintID(b)
		if aid < bid {
			return -1
		}
		if aid > bid {
			return 1
		}
		return 0
	})

	return result, changed
}
