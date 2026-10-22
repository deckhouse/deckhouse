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

package controlplaneoperation

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	etcdclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// On a graceful stop the etcd leader hands its leadership to the member it has been connected to
// the longest, whatever node that member runs on. The approver restarts the leader last
// (control-plane-approver/leader.go), so an etcd arbiter restarted earlier in the rollout than the
// other masters takes the leadership, and every write then waits for the arbiter node.
// Before the new manifest restarts the local member, the operation moves the leadership from it to
// a master. Without arbiters the etcd handover lands on a master anyway and is left alone.
// Best effort: any doubt or error leaves the handover to etcd and never fails the operation.

const (
	etcdLeaderMoveTimeout = 15 * time.Second
	// etcdPeerStatusTimeout bounds the check that a master member can take the leadership.
	etcdPeerStatusTimeout = 2 * time.Second
)

// moveEtcdLeaderToMaster moves the etcd leadership from the local member to a master member when
// the cluster has arbiters and the manifest write is about to restart the local member.
func (r *Reconciler) moveEtcdLeaderToMaster(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, logger *log.Logger) {
	// Desired state already on disk: kubelet will not restart the member.
	if matches, err := manifestMatchesDesired(op); err != nil || matches {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, etcdLeaderMoveTimeout)
	defer cancel()

	// MoveLeader must be sent to the leader, so the client talks to the local member only.
	endpoint := etcd.GetClientURL("127.0.0.1")
	cli, err := etcdclient.NewForEndpoints([]string{endpoint}, constants.KubernetesPkiPath)
	if err != nil {
		logger.Warn("cannot create etcd client, the etcd leadership is left to etcd", log.Err(err))
		return
	}
	defer cli.Close()

	status, err := cli.Status(ctx, endpoint)
	if err != nil {
		logger.Warn("cannot read local etcd member status, the etcd leadership is left to etcd", log.Err(err))
		return
	}
	self := status.Header.GetMemberId()
	if status.Leader != self {
		return
	}

	resp, err := cli.Raw().MemberList(ctx)
	if err != nil {
		logger.Warn("cannot list etcd members, the etcd leadership is left to etcd", log.Err(err))
		return
	}
	members := make([]etcdMemberInfo, 0, len(resp.Members))
	for _, m := range resp.Members {
		members = append(members, etcdMemberInfo{ID: m.ID, Name: m.Name, ClientURLs: m.ClientURLs, IsLearner: m.IsLearner})
	}

	masters := &corev1.NodeList{}
	if err := r.apiReader.List(ctx, masters, client.MatchingLabels{constants.ControlPlaneNodeLabelKey: ""}); err != nil {
		logger.Warn("cannot list master nodes, the etcd leadership is left to etcd", log.Err(err))
		return
	}
	masterNames := make(map[string]bool, len(masters.Items))
	for i := range masters.Items {
		masterNames[masters.Items[i].Name] = true
	}

	transferees := etcdLeaderTransferees(self, members, masterNames)
	for _, m := range transferees {
		if !followsLeader(ctx, cli, m, self) {
			continue
		}
		if _, err := cli.Raw().MoveLeader(ctx, m.ID); err != nil {
			logger.Warn("cannot move etcd leadership to a master, it is left to etcd", slog.String("transferee", m.Name), log.Err(err))
			return
		}
		logger.Info("moved etcd leadership to a master before the local member restarts", slog.String("transferee", m.Name))
		return
	}
	if len(transferees) > 0 {
		logger.Warn("no master etcd member can take the leadership, it is left to etcd")
	}
}

// etcdLeaderTransferees returns the voting members on masters, other than self, in name order.
// It returns none when every voting member, self included, runs on a master: without an arbiter
// the etcd handover on stop already picks a master.
func etcdLeaderTransferees(self uint64, members []etcdMemberInfo, masters map[string]bool) []etcdMemberInfo {
	var onMasters []etcdMemberInfo
	hasArbiter := false
	for _, m := range members {
		switch {
		case m.IsLearner:
		case !masters[m.Name]:
			hasArbiter = true
		case m.ID != self:
			onMasters = append(onMasters, m)
		}
	}
	if !hasArbiter {
		return nil
	}
	slices.SortFunc(onMasters, func(a, b etcdMemberInfo) int { return strings.Compare(a.Name, b.Name) })
	return onMasters
}

// followsLeader reports whether the member answers and sees leader as its leader, so it is
// connected and can take the leadership right away.
func followsLeader(ctx context.Context, cli etcdclient.Interface, m etcdMemberInfo, leader uint64) bool {
	if len(m.ClientURLs) == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, etcdPeerStatusTimeout)
	defer cancel()
	status, err := cli.Status(ctx, m.ClientURLs[0])
	return err == nil && status.Leader == leader && len(status.Errors) == 0
}
