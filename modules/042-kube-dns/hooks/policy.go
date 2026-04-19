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
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type k8sNode struct {
	Name string
	Role string
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var role string

	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("cannot convert get nodes: %v", err)
	}

	// We need special order here
	// Firstly we select master node with label node-role.kubernetes.io/control-plane
	_, ok := node.ObjectMeta.Labels["node-role.kubernetes.io/control-plane"]
	if ok {
		role = "master"
	} else {
		keys := make([]string, 0, len(node.ObjectMeta.Labels))
		for k := range node.ObjectMeta.Labels {
			keys = append(keys, k)
		}

		// Sorting keys, setting alphabet order
		// We prefer deckhouse labels for nodes.
		sort.Strings(keys)

		for _, k := range keys {
			// After that we select deckhouse node roles with node-role.deckhouse.io/*
			if strings.HasPrefix(k, "node-role.deckhouse.io/") {
				role = strings.Split(k, "/")[1]
				break
			}
			// In the end we try to use node-role.kubernetes.io/* label to specify node role.
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				role = strings.Split(k, "/")[1]
				break
			}
		}
	}

	return k8sNode{
		Name: node.Name,
		Role: strings.ToLower(role),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/kube-dns/discover_webhook_certs",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_roles",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
		},
	},
}, setKubeDNSPolicy)

func setKubeDNSPolicy(_ context.Context, input *go_hook.HookInput) error {
	nodes := input.Snapshots.Get("node_roles")
	nodesRolesCounters := make(map[string]int)

	for node, err := range sdkobjectpatch.SnapshotIter[k8sNode](nodes) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'node_roles' snapshot: %w", err)
		}

		nodesRolesCounters[node.Role]++
	}

	switch {
	case nodesRolesCounters["kube-dns"] > 0:
		input.Values.Set("kubeDns.internal.specificNodeType", "kube-dns")
	case nodesRolesCounters["system"] > 0:
		input.Values.Set("kubeDns.internal.specificNodeType", "system")
	default:
		input.Values.Remove("kubeDns.internal.specificNodeType")
	}

	replicas := 2
	switch {
	case nodesRolesCounters["kube-dns"] > 0:
		replicas = nodesRolesCounters["master"] + nodesRolesCounters["kube-dns"]
	case nodesRolesCounters["system"] > 0:
		replicas = nodesRolesCounters["master"] + nodesRolesCounters["system"]
	case nodesRolesCounters["master"] > 2:
		replicas = nodesRolesCounters["master"]
	}

	// limit coredns replicas quantity to prevent special nodes autoscaling problem
	// This block limits count of kube-dns replicas
	if (nodesRolesCounters["master"] + 2) < replicas {
		replicas = nodesRolesCounters["master"] + 2
	}
	input.Values.Set("kubeDns.internal.replicas", replicas)

	if (nodesRolesCounters["master"] + nodesRolesCounters["system"] + nodesRolesCounters["kube-dns"]) > 1 {
		input.Values.Set("kubeDns.internal.enablePodAntiAffinity", true)
	} else {
		input.Values.Set("kubeDns.internal.enablePodAntiAffinity", false)
	}
	return nil
}
