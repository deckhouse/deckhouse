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
	"fmt"
	"slices"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	etcdclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/approver"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
)

// Leadership of etcd, kube-controller-manager and kube-scheduler moves when the leader restarts.
// Restarting the leader last, after every other instance is updated, moves it once per rollout
// and onto an updated instance:
//   - leaderNodes finds the leaders, and the approver orders their operations last in the stage.
//     It is best effort: a leader that cannot be found leaves its component in name order, which
//     costs only extra leadership moves;
//   - markWaitingLeaders makes the leader's operation wait until every other node is updated, that
//     is until its ControlPlaneNode status carries the checksums the leader's operation applies.
//     Nodes whose CPM has not created their operation yet are waited for too;
//   - the approver gate admits the leader's operation only into an empty slot, so a follower
//     approved alongside it cannot take over the leadership.
//
// The handover itself is quick: the etcd leader hands over its leadership on a graceful stop,
// kube-scheduler releases its Lease on shutdown, and for kube-controller-manager, which does not,
// the CPO controller deletes the Lease once its stopped holder no longer renews it
// (control-plane-operation/kcm_leader.go).

const (
	etcdLeaderTimeout = 2 * time.Second
	// missingOperationTimeout bounds how long the leader's operation waits for a node that is not
	// updated and has no operation of the component: its CPM may be down and never create one.
	missingOperationTimeout = 5 * time.Minute
)

// controlPlane is what leader ordering reads about the control plane nodes.
type controlPlane struct {
	// readyMasters run every component; readyEtcd are the ready masters and arbiters.
	readyMasters, readyEtcd []string
	// cpns are the ControlPlaneNodes, named after their nodes.
	cpns []controlplanev1alpha1.ControlPlaneNode
}

// notUpdatedYet returns the ready nodes running op's component, other than op's own, whose
// ControlPlaneNode status lacks a checksum op applies. A node without a ControlPlaneNode is not updated.
// The checksums come from op rather than from each node's spec: CPC writes the specs one node at
// a time, and a node whose spec is not written yet would look updated.
func (cp controlPlane) notUpdatedYet(op controlplanev1alpha1.ControlPlaneOperation) []string {
	running := cp.readyMasters
	if op.Spec.Component == controlplanev1alpha1.OperationComponentEtcd {
		running = cp.readyEtcd
	}
	var notUpdated []string
	for _, node := range running {
		if node != op.Spec.NodeName && !cp.hasApplied(node, op) {
			notUpdated = append(notUpdated, node)
		}
	}
	return notUpdated
}

// hasApplied reports whether the ControlPlaneNode status of node carries every checksum op
// applies. A node without a ControlPlaneNode has applied nothing.
func (cp controlPlane) hasApplied(node string, op controlplanev1alpha1.ControlPlaneOperation) bool {
	i := slices.IndexFunc(cp.cpns, func(cpn controlplanev1alpha1.ControlPlaneNode) bool { return cpn.Name == node })
	if i < 0 {
		return false
	}
	status := cp.cpns[i].Status.Components.Component(op.Spec.Component)
	if status == nil {
		return false
	}
	matches := func(applied, desired string) bool { return desired == "" || applied == desired }
	return matches(status.Checksums.Config, op.Spec.DesiredConfigChecksum) &&
		matches(status.Checksums.PKI, op.Spec.DesiredPKIChecksum) &&
		matches(status.Checksums.CA, op.Spec.DesiredCAChecksum)
}

// leaderNodes maps etcd, kube-controller-manager and kube-scheduler to their leader nodes when
// their pending operations need ordering. A leader that cannot be found is left out.
func (r *reconciler) leaderNodes(
	ctx context.Context,
	ops []controlplanev1alpha1.ControlPlaneOperation,
) map[controlplanev1alpha1.OperationComponent]approver.Leader {
	leaders := make(map[controlplanev1alpha1.OperationComponent]approver.Leader)

	if canLookUpEtcdLeader(ops) {
		if leader, err := r.etcdLeader(ctx); err != nil {
			log.FromContext(ctx).Info("approving etcd operations in name order: cannot find the etcd leader", "error", err.Error())
		} else {
			leaders[controlplanev1alpha1.OperationComponentEtcd] = approver.Leader{Node: leader}
		}
	}

	if slices.ContainsFunc(ops, isPendingSkewSensitive) {
		for _, component := range []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeControllerManager,
			controlplanev1alpha1.OperationComponentKubeScheduler,
		} {
			if leader := r.leaseHolderNode(ctx, ops, component); leader != "" {
				leaders[component] = approver.Leader{Node: leader}
			}
		}
	}
	return leaders
}

// markWaitingLeaders sets Waits on the leaders whose pending operation has to wait for other nodes
// to be updated, and reports whether to re-check them: one of them waits for a node that has no
// operation of the component, and nothing triggers a reconcile when missingOperationTimeout ends.
func markWaitingLeaders(
	ctx context.Context,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	leaders map[controlplanev1alpha1.OperationComponent]approver.Leader,
	cp controlPlane,
	now time.Time,
) bool {
	recheck := false
	for component, leader := range leaders {
		i := slices.IndexFunc(ops, func(op controlplanev1alpha1.ControlPlaneOperation) bool {
			return !op.Spec.Approved && op.Spec.Component == component && op.Spec.NodeName == leader.Node &&
				appliesChecksums(op)
		})
		if i < 0 {
			continue
		}
		leaderOp := ops[i]
		giveUpAt := leaderOp.CreationTimestamp.Add(missingOperationTimeout)

		for _, node := range cp.notUpdatedYet(leaderOp) {
			switch {
			case operations.HasActiveOperation(ops, component, func(op *controlplanev1alpha1.ControlPlaneOperation) bool {
				return op.Spec.NodeName == node && !op.IsObserveOnlyOperation()
			}):
				leader.Waits = true
			case now.Before(giveUpAt):
				leader.Waits = true
				recheck = true
			default:
				log.FromContext(ctx).Info("not waiting for a node that is not updated and has no operation",
					"operation", leaderOp.Name, "node", node, "timeout", missingOperationTimeout.String())
			}
		}
		if leader.Waits {
			log.FromContext(ctx).Info("leader operation waits for other nodes to be updated", "operation", leaderOp.Name)
			leaders[component] = leader
		}
	}
	return recheck
}

// appliesChecksums reports whether op carries desired checksums. An operation without them, such
// as an etcd defrag, has nothing to wait for and must not stand in for the leader's update listed
// after it.
func appliesChecksums(op controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.Spec.DesiredConfigChecksum != "" || op.Spec.DesiredPKIChecksum != "" || op.Spec.DesiredCAChecksum != ""
}

// leaseHolderNode returns the node holding the leader election Lease of component, or "" when the
// Lease cannot be read or is free.
func (r *reconciler) leaseHolderNode(
	ctx context.Context,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	component controlplanev1alpha1.OperationComponent,
) string {
	lease := &coordinationv1.Lease{}
	key := operations.LeaderLeaseKey(component)
	if err := r.apiReader.Get(ctx, key, lease); err != nil {
		log.FromContext(ctx).V(1).Info("cannot read leader Lease", "lease", key.Name, "error", err.Error())
		return ""
	}
	holder := ptr.Deref(lease.Spec.HolderIdentity, "")
	if holder == "" {
		return ""
	}
	leader := operations.LeaseHolderNode(holder)
	if !slices.ContainsFunc(ops, func(op controlplanev1alpha1.ControlPlaneOperation) bool {
		return isPendingSkewSensitive(op) && op.Spec.Component == component && op.Spec.NodeName == leader
	}) {
		// Expected when the leader is already updated; otherwise the hostname differs from the Node name.
		log.FromContext(ctx).V(1).Info("leader Lease holder matches no pending operation", "lease", key.Name, "holderIdentity", holder)
	}
	return leader
}

// localEtcdLeader asks the etcd member on this node which member leads the cluster and returns its
// name, which is the name of its node (--name in etcd.yaml.tpl). It is asked only while no etcd
// operation is in flight, so the local member is not being restarted.
func localEtcdLeader(ctx context.Context) (string, error) {
	endpoint := etcd.GetClientURL("127.0.0.1")
	cli, err := etcdclient.NewForEndpoints([]string{endpoint}, constants.KubernetesPkiPath)
	if err != nil {
		return "", fmt.Errorf("create etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(ctx, etcdLeaderTimeout)
	defer cancel()

	status, err := cli.Status(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("get etcd status: %w", err)
	}
	if status.Leader == 0 {
		return "", errors.New("local etcd member sees no leader")
	}

	members, err := cli.Raw().MemberList(ctx)
	if err != nil {
		return "", fmt.Errorf("list etcd members: %w", err)
	}
	for _, member := range members.Members {
		if member.ID == status.Leader {
			return member.GetName(), nil
		}
	}
	return "", fmt.Errorf("etcd leader %x is not in the member list", status.Leader)
}

// canLookUpEtcdLeader reports whether to ask etcd for its leader: an etcd operation awaits approval
// and none is in flight, so the local member is not being restarted.
func canLookUpEtcdLeader(ops []controlplanev1alpha1.ControlPlaneOperation) bool {
	var awaitsApproval, inFlight bool
	for i := range ops {
		op := &ops[i]
		if op.Spec.Component != controlplanev1alpha1.OperationComponentEtcd || op.IsObserveOnlyOperation() || op.IsTerminal() {
			continue
		}
		if op.Spec.Approved {
			inFlight = true
		} else {
			awaitsApproval = true
		}
	}
	return awaitsApproval && !inFlight
}
