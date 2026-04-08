package drain

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Drainer struct {
	Client client.Client
}

func (d *Drainer) DrainNode(ctx context.Context, node *corev1.Node) error {
	if err := d.cordonNode(ctx, node); err != nil {
		return fmt.Errorf("cordon node %s: %w", node.Name, err)
	}

	if err := d.evictPods(ctx, node); err != nil {
		return fmt.Errorf("evict pods from node %s: %w", node.Name, err)
	}

	return nil
}

func (d *Drainer) cordonNode(ctx context.Context, node *corev1.Node) error {
	if node.Spec.Unschedulable {
		return nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	node.Spec.Unschedulable = true
	return d.Client.Patch(ctx, node, patch)
}

func (d *Drainer) evictPods(ctx context.Context, node *corev1.Node) error {
	podList := &corev1.PodList{}
	if err := d.Client.List(ctx, podList, client.MatchingFieldsSelector{
		Selector: fields.OneTermEqualSelector("spec.nodeName", node.Name),
	}); err != nil {
		return fmt.Errorf("list pods on node %s: %w", node.Name, err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if shouldSkipPod(pod) {
			continue
		}
		if err := d.evictPod(ctx, pod); err != nil {
			return fmt.Errorf("evict pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

func (d *Drainer) evictPod(ctx context.Context, pod *corev1.Pod) error {
	return d.Client.SubResource("eviction").Create(ctx, pod, &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Eviction",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Spec: corev1.PodSpec{},
	})
}

func shouldSkipPod(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return true
	}
	return false
}

func (d *Drainer) WaitForEviction(ctx context.Context, node *corev1.Node, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		podList := &corev1.PodList{}
		if err := d.Client.List(ctx, podList, client.MatchingFieldsSelector{
			Selector: fields.OneTermEqualSelector("spec.nodeName", node.Name),
		}); err != nil {
			return fmt.Errorf("list pods on node %s: %w", node.Name, err)
		}

		active := 0
		for i := range podList.Items {
			if !shouldSkipPod(&podList.Items[i]) {
				active++
			}
		}
		if active == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for pods to be evicted from node %s", node.Name)
}
