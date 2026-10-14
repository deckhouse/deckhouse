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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/approver"
	"control-plane-manager/internal/constants"
)

var testNow = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

func TestMarkWaitingLeaders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	etcd := controlplanev1alpha1.OperationComponentEtcd
	// waits runs markWaitingLeaders for the etcd leader n1 and reports whether it waits, and whether
	// it is to be re-checked, waiting for a node without an operation.
	waits := func(ops []controlplanev1alpha1.ControlPlaneOperation, cp controlPlane, now time.Time) (bool, bool) {
		leaders := map[controlplanev1alpha1.OperationComponent]approver.Leader{etcd: {Node: "n1"}}
		recheck := markWaitingLeaders(ctx, ops, leaders, cp, now)
		return leaders[etcd].Waits, recheck
	}
	// leaderOp is the pending etcd operation on the leader n1, created age before testNow.
	leaderOp := func(age time.Duration) controlplanev1alpha1.ControlPlaneOperation {
		op := testDesiredOperation("e1", "n1", etcd)
		op.CreationTimestamp = metav1.NewTime(testNow.Add(-age))
		return op
	}
	// threeNodes is n1..n3 with n2 and n3 having applied the given etcd config checksums.
	threeNodes := func(n2, n3 string) controlPlane {
		return controlPlane{
			readyEtcd: []string{"n1", "n2", "n3"},
			cpns:      []controlplanev1alpha1.ControlPlaneNode{*testCPN("n1", "old"), *testCPN("n2", n2), *testCPN("n3", n3)},
		}
	}

	t.Run("waits for a node whose operation is in flight", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			leaderOp(time.Hour),
			newOperation("e3", "n3", etcd, true),
		}
		wait, recheck := waits(ops, threeNodes("new", "old"), testNow)
		require.True(t, wait)
		require.False(t, recheck, "a node with an operation is waited for without a timeout")
	})

	t.Run("waits for a node without an operation until the timeout", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{leaderOp(time.Minute)}
		giveUpAt := testNow.Add(missingOperationTimeout - time.Minute)

		wait, recheck := waits(ops, threeNodes("new", "old"), giveUpAt.Add(-time.Nanosecond))
		require.True(t, wait)
		require.True(t, recheck)

		wait, recheck = waits(ops, threeNodes("new", "old"), giveUpAt)
		require.False(t, wait)
		require.False(t, recheck)
	})

	t.Run("does not wait once every other node is updated", func(t *testing.T) {
		t.Parallel()
		wait, recheck := waits([]controlplanev1alpha1.ControlPlaneOperation{leaderOp(0)}, threeNodes("new", "new"), testNow)
		require.False(t, wait)
		require.False(t, recheck)
	})

	t.Run("a defrag listed before the leader's operation does not hide it", func(t *testing.T) {
		t.Parallel()
		defrag := newOperation("d1", "n1", etcd, false)
		defrag.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepDefragEtcd, controlplanev1alpha1.StepWaitPodReady}
		ops := []controlplanev1alpha1.ControlPlaneOperation{defrag, leaderOp(0)}
		wait, recheck := waits(ops, threeNodes("old", "old"), testNow)
		require.True(t, wait)
		require.True(t, recheck)
	})

	t.Run("a node without a ControlPlaneNode is not updated", func(t *testing.T) {
		t.Parallel()
		cp := controlPlane{readyEtcd: []string{"n1", "n2"}, cpns: []controlplanev1alpha1.ControlPlaneNode{*testCPN("n1", "old")}}
		wait, _ := waits([]controlplanev1alpha1.ControlPlaneOperation{leaderOp(0)}, cp, testNow)
		require.True(t, wait)
	})

	t.Run("arbiters do not hold the kube-controller-manager leader", func(t *testing.T) {
		t.Parallel()
		kcm := controlplanev1alpha1.OperationComponentKubeControllerManager
		cp := controlPlane{
			readyMasters: []string{"n1", "n2"},
			readyEtcd:    []string{"n1", "n2", "a1"},
			cpns:         []controlplanev1alpha1.ControlPlaneNode{*testCPN("n1", "old"), *testCPN("n2", "new")},
		}
		ops := []controlplanev1alpha1.ControlPlaneOperation{testDesiredOperation("k1", "n1", kcm)}
		leaders := map[controlplanev1alpha1.OperationComponent]approver.Leader{kcm: {Node: "n1"}}
		markWaitingLeaders(ctx, ops, leaders, cp, testNow)
		require.False(t, leaders[kcm].Waits)
	})
}

func TestCanLookUpEtcdLeader(t *testing.T) {
	t.Parallel()

	etcdOp := func(name, node string, approved bool) controlplanev1alpha1.ControlPlaneOperation {
		op := testDesiredOperation(name, node, controlplanev1alpha1.OperationComponentEtcd)
		op.Spec.Approved = approved
		return op
	}
	observe := etcdOp("observe", "n3", true)
	observe.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}

	for name, tc := range map[string]struct {
		ops  []controlplanev1alpha1.ControlPlaneOperation
		want bool
	}{
		"pending on two nodes":              {ops: []controlplanev1alpha1.ControlPlaneOperation{etcdOp("e1", "n1", false), etcdOp("e2", "n2", false)}, want: true},
		"pending on one node":               {ops: []controlplanev1alpha1.ControlPlaneOperation{etcdOp("e1", "n1", false)}, want: true},
		"etcd operation in flight":          {ops: []controlplanev1alpha1.ControlPlaneOperation{etcdOp("e1", "n1", false), etcdOp("e2", "n2", true)}},
		"observe-only operation is ignored": {ops: []controlplanev1alpha1.ControlPlaneOperation{etcdOp("e1", "n1", false), observe}, want: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, canLookUpEtcdLeader(tc.ops))
		})
	}
}

func TestLeaderNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// etcdLeader is nil: asking etcd panics.

	t.Run("does not ask etcd without a pending etcd operation", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, (&reconciler{}).leaderNodes(ctx, nil))
	})

	t.Run("does not ask etcd while an etcd operation is in flight", func(t *testing.T) {
		t.Parallel()
		ops := []controlplanev1alpha1.ControlPlaneOperation{
			newOperation("e1", "n1", controlplanev1alpha1.OperationComponentEtcd, false),
			newOperation("e2", "n2", controlplanev1alpha1.OperationComponentEtcd, true),
		}
		require.Empty(t, (&reconciler{}).leaderNodes(ctx, ops))
	})
}

func TestReconciler_LeaseLeaderLast(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	run := func(t *testing.T, extra ...client.Object) map[string]bool {
		t.Helper()
		k1 := newOperation("k1", "n1", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		k2 := newOperation("k2", "n2", controlplanev1alpha1.OperationComponentKubeControllerManager, false)
		cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(append([]client.Object{
			testControlPlaneNode("n1"),
			testControlPlaneNode("n2"),
			testConfigSecret("1.36"),
			testAPIServerPod("n1", "1.36"),
			testAPIServerPod("n2", "1.36"),
			&k1,
			&k2,
		}, extra...)...).Build()

		_, err := newReconciler(cl, cl).Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)

		approved := map[string]bool{}
		for _, name := range []string{"k1", "k2"} {
			var updated controlplanev1alpha1.ControlPlaneOperation
			require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: name, Namespace: constants.KubeSystemNamespace}, &updated))
			approved[name] = updated.Spec.Approved
		}
		return approved
	}

	t.Run("approves the leader node last", func(t *testing.T) {
		t.Parallel()
		lease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-controller-manager", Namespace: constants.KubeSystemNamespace},
			Spec:       coordinationv1.LeaseSpec{HolderIdentity: ptr.To("n1_0f1e2d3c")},
		}
		require.Equal(t, map[string]bool{"k1": false, "k2": true}, run(t, lease))
	})

	t.Run("falls back to name order without a Lease", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, map[string]bool{"k1": true, "k2": false}, run(t))
	})
}

func TestReconciler_EtcdLeaderLast(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	leaderN1 := func(context.Context) (string, error) {
		return "n1", nil
	}

	etcdOp := func(name, node string, age time.Duration) *controlplanev1alpha1.ControlPlaneOperation {
		op := testDesiredOperation(name, node, controlplanev1alpha1.OperationComponentEtcd)
		op.CreationTimestamp = metav1.NewTime(time.Now().Add(-age))
		return &op
	}
	// defragOp is a pending etcd operation created now that, like DefragEtcd, changes no checksum.
	defragOp := func(name, node string) *controlplanev1alpha1.ControlPlaneOperation {
		op := newOperation(name, node, controlplanev1alpha1.OperationComponentEtcd, false)
		op.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepDefragEtcd, controlplanev1alpha1.StepWaitPodReady}
		op.CreationTimestamp = metav1.Now()
		return &op
	}
	// run reconciles nodes n1..n3, a pending kube-apiserver operation on n1 and objs, and returns
	// the result and the approved operations.
	run := func(t *testing.T, etcdLeader func(context.Context) (string, error), objs ...client.Object) (reconcile.Result, []string) {
		t.Helper()
		a1 := newOperation("a1", "n1", controlplanev1alpha1.OperationComponentKubeAPIServer, false)
		cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(append([]client.Object{
			testControlPlaneNode("n1"), testControlPlaneNode("n2"), testControlPlaneNode("n3"), &a1,
		}, objs...)...).Build()
		r := newReconciler(cl, cl)
		r.etcdLeader = etcdLeader

		res, err := r.Reconcile(ctx, reconcile.Request{})
		require.NoError(t, err)
		return res, approvedOperations(t, cl)
	}

	t.Run("approves the etcd leader node last", func(t *testing.T) {
		t.Parallel()
		res, approved := run(t, leaderN1, etcdOp("e1", "n1", 0), etcdOp("e2", "n2", 0), etcdOp("e3", "n3", 0))
		require.Zero(t, res.RequeueAfter)
		require.Equal(t, []string{"e2"}, approved)
	})

	t.Run("approves in name order while the etcd leader is unknown", func(t *testing.T) {
		t.Parallel()
		res, approved := run(t, noEtcdLeader, etcdOp("e1", "n1", 0), etcdOp("e2", "n2", 0), etcdOp("e3", "n3", 0))
		require.Zero(t, res.RequeueAfter)
		require.Equal(t, []string{"e1"}, approved)
	})

	t.Run("the leader waits for a node that has not created its operation yet", func(t *testing.T) {
		t.Parallel()
		res, approved := run(t, leaderN1, etcdOp("e1", "n1", time.Minute), testCPN("n1", "old"), testCPN("n2", "new"), testCPN("n3", "old"))
		require.Empty(t, approved, "kube-apiserver waits for the etcd leader too")
		require.Equal(t, recheckInterval, res.RequeueAfter)
	})

	t.Run("approves the leader once every other node is updated", func(t *testing.T) {
		t.Parallel()
		_, approved := run(t, leaderN1, etcdOp("e1", "n1", 0), testCPN("n1", "old"), testCPN("n2", "new"), testCPN("n3", "new"))
		require.Equal(t, []string{"e1"}, approved)
	})

	t.Run("stops waiting for a node without an operation after the timeout", func(t *testing.T) {
		t.Parallel()
		_, approved := run(t, leaderN1, etcdOp("e1", "n1", missingOperationTimeout+time.Minute), testCPN("n1", "old"), testCPN("n2", "new"), testCPN("n3", "old"))
		require.Equal(t, []string{"e1"}, approved)
	})

	t.Run("approves the etcd leader node last for operations that change no checksum", func(t *testing.T) {
		t.Parallel()
		res, approved := run(t, leaderN1, defragOp("e1", "n1"), defragOp("e2", "n2"), defragOp("e3", "n3"), testCPN("n1", "old"), testCPN("n2", "old"), testCPN("n3", "old"))
		require.Zero(t, res.RequeueAfter)
		require.Equal(t, []string{"e2"}, approved)
	})

	t.Run("does not hold the leader's operation that changes no checksum", func(t *testing.T) {
		t.Parallel()
		res, approved := run(t, leaderN1, defragOp("e1", "n1"), testCPN("n1", "old"), testCPN("n2", "old"), testCPN("n3", "old"))
		require.Zero(t, res.RequeueAfter)
		require.Equal(t, []string{"e1"}, approved)
	})
}

// TestReconciler_StaleCache locks that the approver decides on a live list of operations. The cache
// may still show the approval of the previous reconcile as pending; a reconcile that orders etcd
// operations differently, here because it cannot find the etcd leader, would then approve a second
// etcd restart.
func TestReconciler_StaleCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	etcd := controlplanev1alpha1.OperationComponentEtcd
	e1, e2, e3 := newOperation("e1", "n1", etcd, false), newOperation("e2", "n2", etcd, false), newOperation("e3", "n3", etcd, false)
	cl := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
		testControlPlaneNode("n1"), testControlPlaneNode("n2"), testControlPlaneNode("n3"), &e1, &e2, &e3,
	).Build()

	// The cache never catches up: it lists the operations as they were before any approval.
	var stale controlplanev1alpha1.ControlPlaneOperationList
	require.NoError(t, cl.List(ctx, &stale))
	cached := interceptor.NewClient(cl, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if ops, ok := list.(*controlplanev1alpha1.ControlPlaneOperationList); ok {
				stale.DeepCopyInto(ops)
				return nil
			}
			return c.List(ctx, list, opts...)
		},
	})
	r := newReconciler(cached, cl)

	r.etcdLeader = func(context.Context) (string, error) { return "n1", nil }
	_, err := r.Reconcile(ctx, reconcile.Request{})
	require.NoError(t, err)
	require.Equal(t, []string{"e2"}, approvedOperations(t, cl), "the leader n1 goes last")

	r.etcdLeader = noEtcdLeader
	_, err = r.Reconcile(ctx, reconcile.Request{})
	require.NoError(t, err)
	require.Equal(t, []string{"e2"}, approvedOperations(t, cl), "one etcd operation at a time")
}

// noEtcdLeader stands in for an etcd member that cannot be reached.
func noEtcdLeader(context.Context) (string, error) {
	return "", errors.New("connection refused")
}

// approvedOperations lists the names of the approved operations.
func approvedOperations(t *testing.T, cl client.Client) []string {
	t.Helper()
	var ops controlplanev1alpha1.ControlPlaneOperationList
	require.NoError(t, cl.List(context.Background(), &ops))
	var approved []string
	for _, op := range ops.Items {
		if op.Spec.Approved {
			approved = append(approved, op.Name)
		}
	}
	return approved
}

// testDesiredOperation is a pending operation that applies the config checksum "new".
func testDesiredOperation(name, node string, component controlplanev1alpha1.OperationComponent) controlplanev1alpha1.ControlPlaneOperation {
	op := newOperation(name, node, component, false)
	op.Spec.DesiredConfigChecksum = "new"
	return op
}

// testCPN is the ControlPlaneNode of node with config checksum applied to every component.
func testCPN(node, checksum string) *controlplanev1alpha1.ControlPlaneNode {
	cpn := &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{Name: node, Namespace: constants.KubeSystemNamespace},
	}
	applied := controlplanev1alpha1.Checksums{Config: checksum}
	cpn.Status.Components.Etcd.Checksums = applied
	cpn.Status.Components.KubeAPIServer.Checksums = applied
	cpn.Status.Components.KubeControllerManager.Checksums = applied
	cpn.Status.Components.KubeScheduler.Checksums = applied
	return cpn
}
