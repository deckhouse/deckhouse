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

package kubeclient

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"fencing-agent/internal/domain"
)

type Nodes struct {
	client kubernetes.Interface
}

func NewNodes(client kubernetes.Interface) *Nodes {
	return &Nodes{client: client}
}

// ListNodeGroup returns every Node of the NodeGroup, the local one included.
func (n *Nodes) ListNodeGroup(ctx context.Context, nodeGroup string) ([]domain.Peer, error) {
	selector := labels.SelectorFromSet(labels.Set{domain.NodeGroupLabel: nodeGroup}).String()

	list, err := n.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list nodes by %q: %w", selector, err)
	}

	peers := make([]domain.Peer, 0, len(list.Items))
	for i := range list.Items {
		peers = append(peers, domain.Peer{
			Name: list.Items[i].Name,
			IP:   internalIP(&list.Items[i]),
		})
	}

	return peers, nil
}

func internalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}

	return ""
}
