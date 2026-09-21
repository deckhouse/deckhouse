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
	"go.etcd.io/etcd/api/v3/etcdserverpb"
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
	for _, adr := range node.Status.Addresses {
		switch adr.Type {
		case corev1.NodeInternalIP:
			internalIP = adr.Address

		case corev1.NodeExternalIP:
			externalIP = adr.Address
		}
	}
	etcdNode := recicleEtcdNode{
		Name: node.Name,
	}

	if internalIP != "" {
		etcdNode.IP = internalIP
	} else {
		etcdNode.IP = externalIP
	}

	return etcdNode, nil
}

type recicleEtcdNode struct {
	IP   string
	Name string
}

// etcdMemberNodeIP finds the node an etcd member is running on, and reports
// whether the member still belongs to the cluster at all. Everything this hook
// does to a member hangs off it: a member no node claims is removed from etcd.
//
// The name is only the first way to ask. A member publishes its name when it
// starts, so the name etcd reports lags behind the Node object twice over: a
// member added but not yet started has none, and a renamed node keeps announcing
// the name it booted with until its etcd restarts under the new one. In both
// windows the member is healthy, voting and irreplaceable, and matching on the
// name alone would take it out of the cluster.
//
// The peer URL does not lag: it is the address the member was added with and it
// is what its peers reach it on. Matching on it as well narrows what this hook
// removes to members no master node answers for, by name or by address.
func etcdMemberNodeIP(mem *etcdserverpb.Member, byName map[string]string, nodeIPs map[string]struct{}) (string, bool) {
	if ip, ok := byName[mem.Name]; ok {
		return ip, true
	}

	for _, peerURL := range mem.PeerURLs {
		parsed, err := url.Parse(peerURL)
		if err != nil {
			continue
		}
		host := parsed.Hostname()
		if _, ok := nodeIPs[host]; ok {
			return host, true
		}
	}

	return "", false
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
	discoveredEtcdNodeIPs := make(map[string]struct{}, len(snaps))
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
		discoveredEtcdNodeIPs[node.IP] = struct{}{}
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
	for _, mem := range etcdMembersResp.Members {
		if mem.IsLearner {
			input.Logger.Warn("found learner etcd member, will be skipped", slog.Uint64("member_id", mem.ID), slog.String("member_name", mem.Name))
			continue
		}
		if ip, ok := etcdMemberNodeIP(mem, discoveredEtcdNodesMap, discoveredEtcdNodeIPs); ok {
			etcdVotingMembers = append(etcdVotingMembers, fmt.Sprintf("https://%s:2379", ip))
		}
	}
	input.Values.Set("controlPlaneManager.internal.etcdServers", etcdVotingMembers)

	removeListIDs := make([]uint64, 0)
	for _, mem := range etcdMembersResp.Members {
		if _, ok := etcdMemberNodeIP(mem, discoveredEtcdNodesMap, discoveredEtcdNodeIPs); !ok {
			removeListIDs = append(removeListIDs, mem.ID)
			input.Logger.Warn("added etcd member to remove list", slog.Uint64("member_id", mem.ID), slog.String("member_name", mem.Name), slog.Any("peer_urls", mem.PeerURLs))
		}
	}

	input.Logger.Warn("etcd members to remove", slog.Any("remove_list_i_ds", removeListIDs))

	if len(removeListIDs) == len(etcdMembersResp.Members) {
		return fmt.Errorf("attempting do delete every single member from etcd cluster. Exiting")
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
