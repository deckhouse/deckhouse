/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	CordonAnnotationKey   = "node.deckhouse.io/cordoned-by"
	KubeConfigPath        = "/etc/kubernetes/kubelet.conf"
	CordonAnnotationValue = "shutdown-inhibitor"
)

type Node struct {
	*corev1.Node
	client *Klient
	err    error
}

type nodePatch struct {
	Spec     *nodeSpecPatch     `json:"spec,omitempty"`
	Metadata *nodeMetadataPatch `json:"metadata,omitempty"`
}

type nodeSpecPatch struct {
    Unschedulable bool `json:"unschedulable"`
}

type nodeMetadataPatch struct {
	Annotations map[string]*string `json:"annotations,omitempty"`
}

func (c *Klient) GetNode(ctx context.Context, nodeName string) *Node {
	var (
		node    *corev1.Node
		lastErr error
	)
	attempt := 0
	err := wait.ExponentialBackoffWithContext(ctx, kubeAPIRetryBackoff, func(ctx context.Context) (bool, error) {
		attempt++
		var err error
		node, err = c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			dlog.Warn("get node retry", slog.Int("attempt", attempt), dlog.Err(err))
			lastErr = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if lastErr != nil && !isContextError(err) {
			err = lastErr
		}
		return &Node{err: fmt.Errorf("get node %q: %w", nodeName, err)}
	}
	return &Node{Node: node, client: c}
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

func (n *Node) patch(ctx context.Context, p nodePatch) *Node {
	if n.err != nil {
		return n
	}
	patchB, err := json.Marshal(p)
	if err != nil {
		n.err = fmt.Errorf("marshal patch: %w", err)
		return n
	}

	var (
		node    *corev1.Node
		lastErr error
	)
	attempt := 0
	err = wait.ExponentialBackoffWithContext(ctx, kubeAPIRetryBackoff, func(ctx context.Context) (bool, error) {
		attempt++
		var err error
		node, err = n.client.clientset.CoreV1().Nodes().Patch(
			ctx,
			n.Name,
			types.StrategicMergePatchType,
			patchB,
			metav1.PatchOptions{},
		)
		if err != nil {
			dlog.Warn("patch node retry", slog.Int("attempt", attempt), dlog.Err(err))
			lastErr = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if lastErr != nil && !isContextError(err) {
			err = lastErr
		}
		n.err = fmt.Errorf("patch node %q: %w", n.Name, err)
		return n
	}

	n.Node = node
	return n
}

func (n *Node) Cordon(ctx context.Context) *Node {
	return n.patch(ctx, nodePatch{
		Spec: &nodeSpecPatch{Unschedulable: true},
	})
}

func (n *Node) Uncordon(ctx context.Context) *Node {
	return n.patch(ctx, nodePatch{
		Spec: &nodeSpecPatch{Unschedulable: false},
	})
}

func (n *Node) SetCordonAnnotation(ctx context.Context) *Node {
	value := CordonAnnotationValue
	return n.patch(ctx, nodePatch{
		Metadata: &nodeMetadataPatch{
			Annotations: map[string]*string{
				CordonAnnotationKey: &value,
			},
		},
	})
}

func (n *Node) RemoveCordonAnnotation(ctx context.Context) *Node {
	return n.patch(ctx, nodePatch{
		Metadata: &nodeMetadataPatch{
			Annotations: map[string]*string{
				CordonAnnotationKey: nil,
			},
		},
	})
}

func (n *Node) PatchCondition(ctx context.Context, condType, status, reason, message string) *Node {
	if n.err != nil {
		return n
	}

	patch := fmt.Sprintf(
		`{"status":{"conditions":[{"type":"%s", "status":"%s", "reason":"%s", "message":"%s"}]}}`,
		condType, status, reason, message,
	)

	var lastErr error
	attempt := 0
	err := wait.ExponentialBackoffWithContext(ctx, kubeAPIRetryBackoff, func(ctx context.Context) (bool, error) {
		attempt++
		_, err := n.client.clientset.CoreV1().Nodes().Patch(
			ctx,
			n.Name,
			types.StrategicMergePatchType,
			[]byte(patch),
			metav1.PatchOptions{},
			"status",
		)
		if err != nil {
			dlog.Warn("patch node status retry", slog.Int("attempt", attempt), dlog.Err(err))
			lastErr = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if lastErr != nil && !isContextError(err) {
			err = lastErr
		}
		n.err = fmt.Errorf("patch condition on node %q: %w", n.Name, err)
	}

	return n
}

func (n *Node) Err() error {
	return n.err
}
