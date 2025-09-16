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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "etcd_pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "etcd",
					"tier":      "control-plane",
				},
			},
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "status.phase",
						Operator: "Equals",
						Value:    "Running",
					},
				},
			},
			FilterFunc: etcdPeersFilter,
		},
		// common etcd certificate snapshot
		etcdSecretK8sConfig,
	},
}, dependency.WithExternalDependencies(handleCheckEtcdPeers))

func etcdPeersFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	var peerURL string

outer:
	for _, container := range pod.Spec.Containers {
		if container.Name != "etcd" {
			continue
		}
		cmds := append(container.Command, container.Args...) // nolint: gocritic
		for _, cmd := range cmds {
			cmd = strings.TrimSpace(cmd)
			if strings.HasPrefix(cmd, "--listen-peer-urls") {
				peerURL = strings.TrimPrefix(cmd, "--listen-peer-urls=")
				break outer
			}
		}
	}

	var ip string
	if pod.Spec.HostNetwork {
		ip = pod.Status.HostIP
	} else {
		ip = pod.Status.PodIP
	}

	epod := etcdPod{
		IP:      ip,
		PeerURL: peerURL,
	}

	return epod, nil
}

type etcdPod struct {
	IP      string
	PeerURL string
}

func handleCheckEtcdPeers(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	// Make sense only on single master installations. Maybe we can make no checks on values but
	// just look into real etcd members
	masterNodesRaw, ok := input.Values.GetOk("global.discovery.clusterMasterCount")
	if !ok {
		return nil
	}
	masterNodes := masterNodesRaw.Int()

	if masterNodes > 1 {
		return nil
	}
	etcdPods, err := sdkobjectpatch.UnmarshalToStruct[etcdPod](input.Snapshots, "etcd_pods")
	if err != nil {
		return fmt.Errorf("failed to unmarshal etcd_pods snapshot: %w", err)
	}

	if len(etcdPods) == 0 {
		return nil
	}

	etcdPod := etcdPods[0]
	etcdPodIP := etcdPod.IP
	peerURL := etcdPod.PeerURL

	if peerURL == "" {
		return nil
	}

	// connect to etcd cluster
	clientURL := fmt.Sprintf("https://%s:2379", etcdPodIP)
	etcdcli, err := getETCDClient(input, dc, []string{clientURL})
	if err != nil {
		return err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// get etcd members
	resp, err := etcdcli.MemberList(ctx)
	if err != nil {
		return errors.Wrap(err, "list etcd members failed")
	}

	// we are interested in only single master etcd cluster
	if len(resp.Members) != 1 {
		return nil
	}

	// if peer is equal to desired then do nothing
	var etcdMemberID uint64
	for _, member := range resp.Members {
		for _, peer := range member.PeerURLs {
			if peer == peerURL {
				return nil
			}
		}
		etcdMemberID = member.ID
	}

	// update member peerURL
	_, err = etcdcli.MemberUpdate(ctx, etcdMemberID, []string{peerURL})

	return errors.Wrap(err, "update etcd member failed")
}
