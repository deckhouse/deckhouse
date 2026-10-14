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
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// The hook only fixes the peer URL of a single etcd member and produces no values,
// so its failures must not block other hooks or the module start:
// it runs in a dedicated queue and its bindings are not waited for on synchronization.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/check_etcd_peer_urls",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "etcd_pods",
			ApiVersion:             "v1",
			Kind:                   "Pod",
			WaitForSynchronization: ptr.To(false),
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
		{
			Name:                   "master_nodes",
			ApiVersion:             "v1",
			Kind:                   "Node",
			WaitForSynchronization: ptr.To(false),
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: etcdPeersMasterNodeFilter,
		},
		// common etcd certificate snapshot
		etcdSecretK8sConfigNoSyncWait(),
	},
}, dependency.WithExternalDependencies(handleCheckEtcdPeers))

func etcdSecretK8sConfigNoSyncWait() go_hook.KubernetesConfig {
	cfg := etcdSecretK8sConfig
	cfg.WaitForSynchronization = ptr.To(false)
	return cfg
}

func etcdPeersMasterNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

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

	var ready bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			ready = cond.Status == corev1.ConditionTrue
			break
		}
	}

	epod := etcdPod{
		IP:       ip,
		PeerURL:  peerURL,
		NodeName: pod.Spec.NodeName,
		Ready:    ready,
	}

	return epod, nil
}

type etcdPod struct {
	IP       string
	PeerURL  string
	NodeName string
	Ready    bool
}

func handleCheckEtcdPeers(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	masterNodeNames, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "master_nodes")
	if err != nil {
		return fmt.Errorf("failed to unmarshal master_nodes snapshot: %w", err)
	}

	// Make sense only on single master installations.
	if len(masterNodeNames) != 1 {
		return nil
	}
	masterNodeName := masterNodeNames[0]

	etcdPods, err := sdkobjectpatch.UnmarshalToStruct[etcdPod](input.Snapshots, "etcd_pods")
	if err != nil {
		return fmt.Errorf("failed to unmarshal etcd_pods snapshot: %w", err)
	}

	// Pods on nodes without the control-plane role (e.g. leftovers of a removed master
	// whose static manifest was not deleted yet) and not ready pods are not the target:
	// their etcd may be not a cluster member anymore or may not respond at all.
	var etcdPod *etcdPod
	for i := range etcdPods {
		if etcdPods[i].NodeName == masterNodeName && etcdPods[i].Ready {
			etcdPod = &etcdPods[i]
			break
		}
	}

	if etcdPod == nil {
		return nil
	}

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
