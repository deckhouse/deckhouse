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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

// The CRD's toleration type must serialise exactly like corev1.Toleration: stored objects and
// rendered manifests both depend on it. A renamed field or a drifting json tag breaks that
// silently.
func TestTolerationJSONMatchesCorev1(t *testing.T) {
	cases := []struct {
		name string
		own  controlplanev1alpha1.VirtualControlPlaneToleration
		core corev1.Toleration
	}{
		{
			name: "every field set",
			own: controlplanev1alpha1.VirtualControlPlaneToleration{
				Key: "dedicated.deckhouse.io", Operator: corev1.TolerationOpEqual,
				Value: "vcp", Effect: corev1.TaintEffectNoExecute, TolerationSeconds: ptr.To(int64(30)),
			},
			core: corev1.Toleration{
				Key: "dedicated.deckhouse.io", Operator: corev1.TolerationOpEqual,
				Value: "vcp", Effect: corev1.TaintEffectNoExecute, TolerationSeconds: ptr.To(int64(30)),
			},
		},
		{
			name: "tolerate everything",
			own:  controlplanev1alpha1.VirtualControlPlaneToleration{Operator: corev1.TolerationOpExists},
			core: corev1.Toleration{Operator: corev1.TolerationOpExists},
		},
		{
			name: "zero value omits everything",
			own:  controlplanev1alpha1.VirtualControlPlaneToleration{},
			core: corev1.Toleration{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			own, err := json.Marshal(tc.own)
			require.NoError(t, err)
			core, err := json.Marshal(tc.core)
			require.NoError(t, err)
			require.JSONEq(t, string(core), string(own))
		})
	}
}

func TestApplyVCPPlacement(t *testing.T) {
	t.Run("placement reaches the pod spec", func(t *testing.T) {
		vcp := &controlplanev1alpha1.VirtualControlPlane{}
		vcp.Spec.NodeSelector = map[string]controlplanev1alpha1.LabelValue{"topology.kubernetes.io/zone": "eu-west-1a"}
		vcp.Spec.Tolerations = []controlplanev1alpha1.VirtualControlPlaneToleration{{
			Key: "a", Operator: corev1.TolerationOpEqual, Value: "b",
			Effect: corev1.TaintEffectNoExecute, TolerationSeconds: ptr.To(int64(5)),
		}}

		var spec corev1.PodSpec
		applyVCPPlacement(&spec, vcp)

		require.Equal(t, map[string]string{"topology.kubernetes.io/zone": "eu-west-1a"}, spec.NodeSelector)
		require.Equal(t, []corev1.Toleration{{
			Key: "a", Operator: corev1.TolerationOpEqual, Value: "b",
			Effect: corev1.TaintEffectNoExecute, TolerationSeconds: ptr.To(int64(5)),
		}}, spec.Tolerations)
	})

	// Deployments are compared to the cluster with equality.Semantic.DeepEqual on exactly these two
	// fields, and that tells nil from an empty collection. Turning an absent field into an empty
	// one would make every reconcile see a difference and update forever.
	t.Run("absent placement stays nil", func(t *testing.T) {
		var spec corev1.PodSpec
		applyVCPPlacement(&spec, &controlplanev1alpha1.VirtualControlPlane{})

		require.Nil(t, spec.NodeSelector)
		require.Nil(t, spec.Tolerations)
	})
}

// Placement is substituted into YAML templates by a plain strings.Replacer, so it has to stay
// single-line JSON.
func TestRenderPlacement(t *testing.T) {
	vcp := &controlplanev1alpha1.VirtualControlPlane{}
	vcp.Spec.Tolerations = []controlplanev1alpha1.VirtualControlPlaneToleration{
		{Key: "dedicated.deckhouse.io", Operator: corev1.TolerationOpEqual, Value: "vcp", Effect: corev1.TaintEffectNoSchedule},
	}
	vcp.Spec.NodeSelector = map[string]controlplanev1alpha1.LabelValue{"node-role.deckhouse.io/vcp": ""}

	tolerations, err := renderTolerations(vcp)
	require.NoError(t, err)
	require.Equal(t, `[{"key":"dedicated.deckhouse.io","operator":"Equal","value":"vcp","effect":"NoSchedule"}]`, tolerations)

	nodeSelector, err := renderNodeSelector(vcp)
	require.NoError(t, err)
	require.Equal(t, `{"node-role.deckhouse.io/vcp":""}`, nodeSelector)
}
