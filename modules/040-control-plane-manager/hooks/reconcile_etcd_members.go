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

	// On dual-stack nodes Kubernetes reports both an IPv4 and an IPv6
	// InternalIP. etcd's static manifests advertise only the primary node IP
	// (see candi/control-plane/etcd.yaml.tpl), so prefer the IPv4 address and
	// fall back to whatever single address is available.
	var internalIPv4, internalIPv6, externalIPv4, externalIPv6 string
	for _, adr := range node.Status.Addresses {
		switch adr.Type {
		case corev1.NodeInternalIP:
			if isIPv4(adr.Address) {
				if internalIPv4 == "" {
					internalIPv4 = adr.Address
				}
			} else if internalIPv6 == "" {
				internalIPv6 = adr.Address
			}

		case corev1.NodeExternalIP:
			if isIPv4(adr.Address) {
				if externalIPv4 == "" {
					externalIPv4 = adr.Address
				}
			} else if externalIPv6 == "" {
				externalIPv6 = adr.Address
			}
		}
	}

	etcdNode := recicleEtcdNode{Name: node.Name}
	for _, ip := range []string{internalIPv4, internalIPv6, externalIPv4, externalIPv6} {
		if ip != "" {
			etcdNode.IP = ip
			break
		}
	}

	return etcdNode, nil
}

type recicleEtcdNode struct {
	IP   string
	Name string
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
		etcdServersEndpoints = append(etcdServersEndpoints, etcdEndpoint(node.IP))
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
			input.Logger.Warn("found learner etcd member, will be skipped", slog.Uint64("memberID", mem.ID), slog.String("memberName", mem.Name))
			continue
		}
		if ip, ok := discoveredEtcdNodesMap[mem.Name]; ok {
			etcdVotingMembers = append(etcdVotingMembers, etcdEndpoint(ip))
		}
	}
	input.Values.Set("controlPlaneManager.internal.etcdServers", etcdVotingMembers)

	removeListIDs := make([]uint64, 0)
	for _, mem := range etcdMembersResp.Members {
		if _, ok := discoveredEtcdNodesMap[mem.Name]; !ok {
			removeListIDs = append(removeListIDs, mem.ID)
			input.Logger.Warn("added etcd member to remove list", slog.Uint64("memberID", mem.ID), slog.String("memberName", mem.Name))
		}
	}

	input.Logger.Warn("etcd members to remove", slog.Any("removeListIDs", removeListIDs))

	if len(removeListIDs) == len(etcdMembersResp.Members) {
		return fmt.Errorf("attempting do delete every single member from etcd cluster. Exiting")
	}

	for _, rm := range removeListIDs {
		input.Logger.Warn("removing etcd member", slog.Uint64("memberID", rm))
		_, err = etcdcli.MemberRemove(ctx, rm)
		if err != nil {
			return errors.Wrap(err, "remove etcd member failed")
		}
	}

	return nil
}
