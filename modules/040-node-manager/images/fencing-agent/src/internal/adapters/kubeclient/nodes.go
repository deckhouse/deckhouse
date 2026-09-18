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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"fencing-agent/internal/domain"
)

type Nodes struct{ client kubernetes.Interface }

func NewNodes(client kubernetes.Interface) *Nodes {
	return &Nodes{client: client}
}

func (n *Nodes) GetNode(ctx context.Context, name string) (domain.NodeRecord, error) {
	node, err := n.client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return domain.NodeRecord{}, fmt.Errorf("get node %q: %w: %w", name, domain.ErrNodeNotFound, err)
		}

		return domain.NodeRecord{}, fmt.Errorf("get node %q: %w", name, err)
	}

	return domain.NodeRecord{
		Name:      node.Name,
		UID:       string(node.UID),
		IP:        internalIP(node),
		NodeGroup: nodeSignals(node).NodeGroup,
	}, nil
}

func internalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}

	return ""
}
