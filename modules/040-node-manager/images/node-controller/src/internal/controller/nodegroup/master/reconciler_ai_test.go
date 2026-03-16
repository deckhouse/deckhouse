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

package master

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = deckhousev1.AddToScheme(s)
	return s
}

func newReconciler(cl client.Client, scheme *runtime.Scheme) *NodeGroupMasterReconciler {
	r := &NodeGroupMasterReconciler{}
	r.Base = dynr.Base{
		Client: cl,
		Scheme: scheme,
	}
	return r
}

func TestAI_MasterNodeGroupCreatedWhenNotExists(t *testing.T) {
	s := newTestScheme()

	cl := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: masterNodeGroupName},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify master NodeGroup was created
	created := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: masterNodeGroupName}, created)
	require.NoError(t, err, "master NodeGroup should have been created")

	assert.Equal(t, "master", created.Name)
	// Without d8-cluster-configuration secret, it defaults to CloudPermanent
	assert.Equal(t, deckhousev1.NodeTypeCloudPermanent, created.Spec.NodeType)
	assert.NotNil(t, created.Spec.Disruptions)
	assert.Equal(t, deckhousev1.DisruptionApprovalModeManual, created.Spec.Disruptions.ApprovalMode)
	assert.NotNil(t, created.Spec.NodeTemplate)
	assert.Equal(t, "", created.Spec.NodeTemplate.Labels["node-role.kubernetes.io/control-plane"])
	assert.Equal(t, "", created.Spec.NodeTemplate.Labels["node-role.kubernetes.io/master"])
	require.Len(t, created.Spec.NodeTemplate.Taints, 1)
	assert.Equal(t, "node-role.kubernetes.io/control-plane", created.Spec.NodeTemplate.Taints[0].Key)
	assert.Equal(t, corev1.TaintEffectNoSchedule, created.Spec.NodeTemplate.Taints[0].Effect)
}

func TestAI_MasterNodeGroupCreatedAsStaticForStaticCluster(t *testing.T) {
	s := newTestScheme()

	// Create d8-cluster-configuration secret with clusterType: Static
	clusterConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-cluster-configuration",
			Namespace: "kube-system",
		},
		Data: map[string][]byte{
			"clusterType": []byte("Static"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(clusterConfigSecret).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: masterNodeGroupName},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	created := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: masterNodeGroupName}, created)
	require.NoError(t, err, "master NodeGroup should have been created")

	assert.Equal(t, deckhousev1.NodeTypeStatic, created.Spec.NodeType)
}

func TestAI_MasterNodeGroupAlreadyExists(t *testing.T) {
	s := newTestScheme()

	existingMasterNG := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "master",
			ResourceVersion: "12345",
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: deckhousev1.NodeTypeCloudPermanent,
			Disruptions: &deckhousev1.DisruptionsSpec{
				ApprovalMode: deckhousev1.DisruptionApprovalModeAutomatic,
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(existingMasterNG).
		Build()

	r := newReconciler(cl, s)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: masterNodeGroupName},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the existing master NodeGroup was NOT modified
	existing := &deckhousev1.NodeGroup{}
	err = cl.Get(context.Background(), client.ObjectKey{Name: masterNodeGroupName}, existing)
	require.NoError(t, err)

	// It should still have Automatic mode, not Manual (which is the default for new creation)
	assert.Equal(t, deckhousev1.DisruptionApprovalModeAutomatic, existing.Spec.Disruptions.ApprovalMode,
		"existing master NodeGroup should not be modified")
}

func TestAI_NonMasterNodeGroupIgnored(t *testing.T) {
	workerNG := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
	}

	// Filtering of non-master NodeGroups is done in SetupForPredicates, not in Reconcile.
	r := &NodeGroupMasterReconciler{}
	preds := r.SetupForPredicates()
	require.Len(t, preds, 1)

	assert.False(t, preds[0].Create(event.CreateEvent{Object: workerNG}),
		"predicate should reject non-master NodeGroup")

	// Also verify that a master NodeGroup passes the predicate.
	masterNG := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "master"},
	}
	assert.True(t, preds[0].Create(event.CreateEvent{Object: masterNG}),
		"predicate should accept master NodeGroup")
}
