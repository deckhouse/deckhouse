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
	"reflect"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func getTemplateLabels(ng *v1.NodeGroup) map[string]string {
	if ng.Spec.NodeTemplate == nil || ng.Spec.NodeTemplate.Labels == nil {
		return map[string]string{}
	}
	return cloneStringMap(ng.Spec.NodeTemplate.Labels)
}

func getTemplateAnnotations(ng *v1.NodeGroup) map[string]string {
	if ng.Spec.NodeTemplate == nil || ng.Spec.NodeTemplate.Annotations == nil {
		return map[string]string{}
	}
	return cloneStringMap(ng.Spec.NodeTemplate.Annotations)
}

func getTemplateTaints(ng *v1.NodeGroup) []corev1.Taint {
	if ng.Spec.NodeTemplate == nil || len(ng.Spec.NodeTemplate.Taints) == 0 {
		return nil
	}
	return append([]corev1.Taint(nil), ng.Spec.NodeTemplate.Taints...)
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func nodeChanged(a, b *corev1.Node) bool {
	if !reflect.DeepEqual(a.Labels, b.Labels) {
		return true
	}
	if !reflect.DeepEqual(a.Annotations, b.Annotations) {
		return true
	}
	if !reflect.DeepEqual(a.Spec.Taints, b.Spec.Taints) {
		return true
	}
	return false
}

func shouldDisableScaleDown(nodeType v1.NodeType) bool {
	return nodeType == v1.NodeTypeCloudPermanent ||
		nodeType == v1.NodeTypeCloudStatic ||
		nodeType == v1.NodeTypeStatic
}

func hasKey(m map[string]string, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}
