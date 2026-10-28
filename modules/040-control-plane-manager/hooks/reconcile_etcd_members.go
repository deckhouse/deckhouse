/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/reconcile_etcd_members",
	Schedule: []go_hook.ScheduleConfig{
		{
			Crontab: "*/5 * * * *",
			Name:    "reconcicle-etcd-members",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: reconcicleEtcdFilterNode,
		},
		{
			Name:       "etcd_arbiter_node",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node.deckhouse.io/etcd-arbiter": "",
				},
			},
			FilterFunc: reconcicleEtcdFilterNode,
		},
		// common etcd certificate snapshot
		etcdSecretK8sConfig,
	},
}, dependency.WithExternalDependencies(handleRecicleEtcdMembers))

func reconcicleEtcdFilterNode(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(unstructured, &node)
	if err != nil {
		return nil, err
	}

	var internalIP, externalIP string
	addresses := make([]string, 0, len(node.Status.Addresses))
	for _, adr := range node.Status.Addresses {
		addresses = append(addresses, adr.Address)
		switch adr.Type {
		case corev1.NodeInternalIP:
			internalIP = adr.Address

		case corev1.NodeExternalIP:
			externalIP = adr.Address
		}
	}
	etcdNode := recicleEtcdNode{
		Name:      node.Name,
		Addresses: addresses,
	}

	if internalIP != "" {
		etcdNode.IP = internalIP
	} else {
		etcdNode.IP = externalIP
	}

	return etcdNode, nil
}

type recicleEtcdNode struct {
	IP        string
	Name      string
	Addresses []string
}

func handleRecicleEtcdMembers(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	snapsM := input.Snapshots.Get("master_nodes")
	snapsEO := input.Snapshots.Get("etcd_arbiter_node")
	snaps := make([]pkg.Snapshot, 0, len(snapsM)+len(snapsEO))
	snaps = append(snaps, snapsM...)
	snaps = append(snaps, snapsEO...)

	if len(snaps) == 0 {
		input.Logger.Debug("No ETCD Nodes found in snapshot, skipping iteration")
		return nil
	}

	etcdServersEndpoints := make([]string, 0, len(snaps))
	discoveredEtcdNodesMap := make(map[string]string, len(snaps))
	discoveredEtcdNodesAddresses := make(map[string]struct{}, len(snaps))
	for node, err := range sdkobjectpatch.SnapshotIter[recicleEtcdNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over ETCD Nodes snapshots: %v", err)
		}

		if node.Name == "" {
			return fmt.Errorf("node_name should not be empty")
		}
		if node.IP == "" {
			return fmt.Errorf("ip should not be empty")
		}

		discoveredEtcdNodesMap[node.Name] = node.IP
		for _, addr := range node.Addresses {
			discoveredEtcdNodesAddresses[addr] = struct{}{}
		}
		etcdServersEndpoints = append(etcdServersEndpoints, fmt.Sprintf("https://%s:2379", node.IP))
	}

	// etcd
	etcdcli, err := getETCDClient(input, dc, etcdServersEndpoints)
	if err != nil {
		return err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	etcdMembersResp, err := etcdcli.MemberList(ctx)
	if err != nil {
		return errors.Wrap(err, "list etcd members failed")
	}

	etcdVotingMembers := make([]string, 0, len(etcdMembersResp.Members))
	removeListIDs := make([]uint64, 0)
	for _, mem := range etcdMembersResp.Members {
		ip, isKnownNode := discoveredEtcdNodesMap[mem.Name]

		switch {
		// A member gets its name only after its etcd starts. Until then a learner of a joining master is matched by peer URL host.
		// Voters are not exempt: an unstarted voter counts toward quorum, and the join flow promotes only started learners.
		case mem.IsLearner && mem.Name == "" && hasPeerOnKnownNode(mem.PeerURLs, discoveredEtcdNodesAddresses):
			input.Logger.Warn("skipping unstarted etcd learner of a known node", slog.Uint64("member_id", mem.ID), slog.Any("peer_urls", mem.PeerURLs))
		case !isKnownNode:
			removeListIDs = append(removeListIDs, mem.ID)
			input.Logger.Warn("added etcd member to remove list", slog.Uint64("member_id", mem.ID), slog.String("member_name", mem.Name))
		case mem.IsLearner:
			input.Logger.Warn("found learner etcd member, will be skipped", slog.Uint64("member_id", mem.ID), slog.String("member_name", mem.Name))
		default:
			etcdVotingMembers = append(etcdVotingMembers, fmt.Sprintf("https://%s:2379", ip))
		}
	}
	input.Values.Set("controlPlaneManager.internal.etcdServers", etcdVotingMembers)

	input.Logger.Warn("etcd members to remove", slog.Any("remove_list_i_ds", removeListIDs))

	// Learners do not count: the cluster cannot work without a voting member.
	if len(etcdVotingMembers) == 0 {
		return fmt.Errorf("attempting to delete every voting member from etcd cluster. Exiting")
	}

	for _, rm := range removeListIDs {
		input.Logger.Warn("removing etcd member", slog.Uint64("member_id", rm))
		_, err = etcdcli.MemberRemove(ctx, rm)
		if err != nil {
			return errors.Wrap(err, "remove etcd member failed")
		}
	}

	return nil
}

// hasPeerOnKnownNode reports whether any peer URL (e.g. "https://192.168.1.4:2380") points to one of the node addresses.
func hasPeerOnKnownNode(peerURLs []string, nodeAddresses map[string]struct{}) bool {
	for _, peerURL := range peerURLs {
		parsed, err := url.Parse(peerURL)
		if err != nil {
			continue
		}

		peerIP := parsed.Hostname()
		if _, isKnown := nodeAddresses[peerIP]; isKnown {
			return true
		}
	}

	return false
}
