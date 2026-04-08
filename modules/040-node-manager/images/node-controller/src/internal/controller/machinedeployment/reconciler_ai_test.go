//go:build ai_tests

/*
Copyright 2025 Flant JSC

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

package machinedeployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
)

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = deckhousev1.AddToScheme(s)
	_ = mcmv1alpha1.AddToScheme(s)
	return s
}

func makeNodeGroup(name string, minPerZone, maxPerZone int32) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudEphemeral,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone: minPerZone,
				MaxPerZone: maxPerZone,
				ClassReference: deckhousev1.ClassReference{
					Kind: "AWSInstanceClass",
					Name: "worker",
				},
			},
		},
	}
}

func makeMD(name, ngName string, replicas int32) *mcmv1alpha1.MachineDeployment {
	return &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: machineDeploymentNamespace,
			Labels:    map[string]string{nodeGroupLabel: ngName},
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: replicas,
		},
	}
}

func reconcileMD(t *testing.T, md *mcmv1alpha1.MachineDeployment, ng *deckhousev1.NodeGroup) *mcmv1alpha1.MachineDeployment {
	t.Helper()
	s := newScheme()

	objs := []runtime.Object{md}
	if ng != nil {
		objs = append(objs, ng)
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...).
		Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      md.Name,
			Namespace: md.Namespace,
		},
	})
	require.NoError(t, err)

	got := &mcmv1alpha1.MachineDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      md.Name,
		Namespace: md.Namespace,
	}, got))
	return got
}

// min >= max => replicas = max
func TestAI_MachineDeployment_MinGeMax_ClampToMax(t *testing.T) {
	ng := makeNodeGroup("ng1", 5, 2)
	md := makeMD("md-ng1", "ng1", 1)

	got := reconcileMD(t, md, ng)
	assert.Equal(t, int32(2), got.Spec.Replicas, "when min >= max, replicas should be clamped to max")
}

// replicas == 0 => replicas = min
func TestAI_MachineDeployment_ZeroReplicas_SetToMin(t *testing.T) {
	ng := makeNodeGroup("ng20", 3, 4)
	md := makeMD("md-ng20", "ng20", 0)

	got := reconcileMD(t, md, ng)
	assert.Equal(t, int32(3), got.Spec.Replicas, "zero replicas should be set to min")
}

// replicas < min => replicas = min
func TestAI_MachineDeployment_ReplicasBelowMin_ClampToMin(t *testing.T) {
	ng := makeNodeGroup("ng3", 6, 10)
	md := makeMD("md-ng3", "ng3", 2)

	got := reconcileMD(t, md, ng)
	assert.Equal(t, int32(6), got.Spec.Replicas, "replicas below min should be clamped to min")
}

// replicas > max => replicas = max
func TestAI_MachineDeployment_ReplicasAboveMax_ClampToMax(t *testing.T) {
	ng := makeNodeGroup("ng4", 3, 4)
	md := makeMD("md-ng4", "ng4", 7)

	got := reconcileMD(t, md, ng)
	assert.Equal(t, int32(4), got.Spec.Replicas, "replicas above max should be clamped to max")
}

// min <= replicas <= max => no change
func TestAI_MachineDeployment_ReplicasInRange_NoChange(t *testing.T) {
	ng := makeNodeGroup("ng5", 1, 10)
	md := makeMD("md-ng5", "ng5", 5)

	got := reconcileMD(t, md, ng)
	assert.Equal(t, int32(5), got.Spec.Replicas, "replicas in range should not change")
}

// NodeGroup not found => no change (replicas stay as-is)
func TestAI_MachineDeployment_NGNotFound_NoChange(t *testing.T) {
	md := makeMD("md-ng6", "ng6", 5)

	got := reconcileMD(t, md, nil)
	assert.Equal(t, int32(5), got.Spec.Replicas, "replicas should not change when NG is missing")
}

// MD not found => no error
func TestAI_MachineDeployment_MDNotFound(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()

	r := &Reconciler{}
	r.Client = c

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: machineDeploymentNamespace,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// MD without node-group label => skip
func TestAI_MachineDeployment_NoLabel_Skip(t *testing.T) {
	md := &mcmv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "md-no-label",
			Namespace: machineDeploymentNamespace,
		},
		Spec: mcmv1alpha1.MachineDeploymentSpec{
			Replicas: 5,
		},
	}

	s := newScheme()
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(md).
		Build()

	r := &Reconciler{}
	r.Client = c

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      md.Name,
			Namespace: md.Namespace,
		},
	})
	require.NoError(t, err)

	got := &mcmv1alpha1.MachineDeployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name:      md.Name,
		Namespace: md.Namespace,
	}, got))
	assert.Equal(t, int32(5), got.Spec.Replicas, "MD without label should not be modified")
}

// Static node group with staticInstances.count
func TestAI_MachineDeployment_StaticInstances_ClampToCount(t *testing.T) {
	count := int32(3)
	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "ng-static"},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeStatic,
			StaticInstances: &deckhousev1.StaticInstancesSpec{
				Count: &count,
			},
		},
	}
	md := makeMD("md-static", "ng-static", 5)

	got := reconcileMD(t, md, ng)
	assert.Equal(t, int32(3), got.Spec.Replicas, "replicas should be clamped to staticInstances.count")
}
