// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctrlutils

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
)

// The manager cache takes it as cache.ByObject.Transform.
var _ toolscache.TransformFunc = TransformStripNode

func TestTransformStripNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "worker-0",
			Labels:        map[string]string{"node.deckhouse.io/group": "worker"},
			Annotations:   map[string]string{"node.deckhouse.io/virtualization": "kvm"},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubelet", Operation: metav1.ManagedFieldsOperationUpdate}},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}},
		},
		Status: corev1.NodeStatus{
			Capacity:        corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")},
			NodeInfo:        corev1.NodeSystemInfo{KubeletVersion: "v1.33.4"},
			Images:          []corev1.ContainerImage{{Names: []string{"registry.example.com/app:v1"}, SizeBytes: 1 << 20}},
			Conditions:      []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			VolumesInUse:    []corev1.UniqueVolumeName{"kubernetes.io/csi/driver^volume"},
			VolumesAttached: []corev1.AttachedVolume{{Name: "kubernetes.io/csi/driver^volume"}},
		},
	}

	// Everything but the dropped fields survives: the name and labels the ClusterConfiguration
	// webhook lists by, the taints, the capacity and the annotation the licensing controller reads.
	want := node.DeepCopy()
	want.ManagedFields = nil
	want.Status.Images = nil
	want.Status.Conditions = nil
	want.Status.VolumesInUse = nil
	want.Status.VolumesAttached = nil

	got, err := TransformStripNode(node)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// An object already in the cache may come through again.
	again, err := TransformStripNode(got)
	require.NoError(t, err)
	require.Equal(t, want, again)

	// A tombstone carries an object that was transformed on its way into the cache.
	tombstone := toolscache.DeletedFinalStateUnknown{Key: "worker-0", Obj: node}
	passed, err := TransformStripNode(tombstone)
	require.NoError(t, err)
	require.Equal(t, tombstone, passed)
}
