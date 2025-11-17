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

func handleRecicleEtcdMembers(input *go_hook.HookInput, dc dependency.Container) error {
	snaps := input.NewSnapshots.Get("master_nodes")

	if len(snaps) == 0 {
		input.Logger.Debug("No master Nodes found in snapshot, skipping iteration")
		return nil
	}

	etcdServersEndpoints := make([]string, 0, len(snaps))
	discoveredMasterMap := make(map[string]string, len(snaps))
	for node, err := range sdkobjectpatch.SnapshotIter[recicleEtcdNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'master_nodes' snapshots: %v", err)
		}

		if node.Name == "" {
			return fmt.Errorf("node_name should not be empty")
		}
		if node.IP == "" {
			return fmt.Errorf("ip should not be empty")
		}

		discoveredMasterMap[node.Name] = node.IP
		etcdServersEndpoints = append(etcdServersEndpoints, fmt.Sprintf("https://%s:2379", node.IP))
	}

	input.Values.Set("controlPlaneManager.internal.etcdServers", etcdServersEndpoints)

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

	removeListIDs := make([]uint64, 0)
	for _, mem := range etcdMembersResp.Members {
		if _, ok := discoveredMasterMap[mem.Name]; !ok {
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
