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

package virtualcontrolplaneconfiguration

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

// Manifests are rendered by a plain strings.Replacer, so a placement value has to survive
// substitution at an arbitrary indentation. YAML flow style is single-line and therefore
// indentation-neutral; block style is not. JSON is valid YAML flow, and encoding/json sorts map
// keys, which keeps the rendered config Secret byte-stable across reconciles.

func renderNodeSelector(vcp *controlplanev1alpha1.VirtualControlPlane) (string, error) {
	if len(vcp.Spec.NodeSelector) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(vcp.Spec.NodeSelector)
	if err != nil {
		return "", fmt.Errorf("marshal nodeSelector: %w", err)
	}
	return string(raw), nil
}

func renderTolerations(vcp *controlplanev1alpha1.VirtualControlPlane) (string, error) {
	if len(vcp.Spec.Tolerations) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(vcp.Spec.Tolerations)
	if err != nil {
		return "", fmt.Errorf("marshal tolerations: %w", err)
	}
	return string(raw), nil
}

// applyVCPPlacement is the Go-side equivalent for Deployments built from embedded YAML instead of
// the config Secret templates.
func applyVCPPlacement(spec *corev1.PodSpec, vcp *controlplanev1alpha1.VirtualControlPlane) {
	spec.NodeSelector = maps.Clone(vcp.Spec.NodeSelector)
	spec.Tolerations = slices.Clone(vcp.Spec.Tolerations)
}
