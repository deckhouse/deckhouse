/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	CordonAnnotationKey   = "node.deckhouse.io/cordoned-by"
	KubectlPath           = "/opt/deckhouse/bin/kubectl"
	KubeConfigPath        = "/etc/kubernetes/kubelet.conf"
	CordonAnnotationValue = "shutdown-inhibitor"
)

type Node struct {
	*corev1.Node
	client *Klient
	err    error
}

func (c *Klient) GetNode(ctx context.Context, nodeName string) *Node {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return &Node{err: fmt.Errorf("get node %q: %w", nodeName, err)}
	}
	return &Node{Node: node, client: c}
}

func (n *Node) update(ctx context.Context) *Node {
	if n.err != nil {
		return n
	}
	_, err := n.client.clientset.CoreV1().Nodes().Update(ctx, n.Node, metav1.UpdateOptions{})
	if err != nil {
		n.err = fmt.Errorf("update node %q: %w", n.Name, err)
	}
	return n
}

func (n *Node) Uncordon(ctx context.Context) *Node {
	if n.err != nil {
		return n
	}
	n.Spec.Unschedulable = false
	return n.update(ctx)
}

func (n *Node) Cordon(ctx context.Context) *Node {
	if n.err != nil {
		return n
	}
	n.Spec.Unschedulable = true
	return n.update(ctx)
}

func (n *Node) RemoveCordonAnnotation(ctx context.Context) *Node {
	if n.err != nil {
		return n
	}
	annotations := n.GetAnnotations()
	if annotations == nil {
		return n
	}
	delete(annotations, CordonAnnotationKey)
	n.SetAnnotations(annotations)
	return n.update(ctx)
}

func (n *Node) SetCordonAnnotation(ctx context.Context) *Node {
	if n.err != nil {
		return n
	}
	annotations := n.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[CordonAnnotationKey] = CordonAnnotationValue
	n.SetAnnotations(annotations)
	return n.update(ctx)
}

func (n *Node) GetAnnotationCordonedBy() (string, error) {
	annotations := n.GetAnnotations()
	if annotations == nil {
		return "", n.err
	}
	return annotations[CordonAnnotationKey], n.err
}

func (n *Node) GetConditionByReason(reason string) (corev1.NodeCondition, error) {
	for _, c := range n.Status.Conditions {
		if c.Reason == reason {
			return c, n.err
		}
	}
	return corev1.NodeCondition{}, n.err
}

func (n *Node) PatchCondition(ctx context.Context, condType, status, reason, message string) *Node {
	if n.err != nil {
		return n
	}

	patch := fmt.Sprintf(
		`{"status":{"conditions":[{"type":"%s", "status":"%s", "reason":"%s", "message":"%s"}]}}`,
		condType, status, reason, message,
	)

	_, err := n.client.clientset.CoreV1().Nodes().Patch(
		ctx,
		n.Name,
		types.StrategicMergePatchType,
		[]byte(patch),
		metav1.PatchOptions{},
		"status",
	)
	if err != nil {
		n.err = fmt.Errorf("patch condition on node %q: %w", n.Name, err)
	}

	return n
}

func (n *Node) Err() error {
	return n.err
}
