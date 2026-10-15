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

package controlplaneapprover

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func TestControlPlanePodPredicate(t *testing.T) {
	t.Parallel()
	apiServer := controlplanev1alpha1.OperationComponentKubeAPIServer
	relabeled := testManagerPod("cpm-n1", "n1", testHeldImage, running)
	relabeled.Labels["example"] = "changed"
	proxy := func(state corev1.ContainerState) *corev1.Pod {
		pod := testStaticPod("n1", apiServer, testTemplateImage, state)
		pod.Labels[constants.StaticPodComponentLabelKey] = "kubernetes-api-proxy"
		return pod
	}

	for name, tc := range map[string]struct {
		old, new *corev1.Pod
		want     bool
	}{
		"a holder starts running":      {testManagerPod("cpm-n1", "n1", testHeldImage, pulling), testManagerPod("cpm-n1", "n1", testHeldImage, running), true},
		"a static pod starts running":  {testStaticPod("n1", apiServer, testTemplateImage, pulling), testStaticPod("n1", apiServer, testTemplateImage, running), true},
		"running images do not change": {testManagerPod("cpm-n1", "n1", testHeldImage, running), relabeled, false},
		"another component label":      {proxy(pulling), proxy(running), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, ControlPlanePodPredicate().Update(event.UpdateEvent{ObjectOld: tc.old, ObjectNew: tc.new}))
		})
	}
}
