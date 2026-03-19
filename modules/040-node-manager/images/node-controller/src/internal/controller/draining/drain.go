/*
Copyright 2025 Flant JSC

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

package draining

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// drainNodePods evicts all pods from a node, respecting the context timeout.
// It mirrors the behavior of the drain.RunNodeDrain helper used in the hook.
func drainNodePods(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	logger := log.FromContext(ctx)

	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String(),
	})
	if err != nil {
		return fmt.Errorf("listing pods on node %s: %w", nodeName, err)
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		errs        []error
		skippedDS   int
		skippedMirr int
		toEvict     int
	)

	for i := range podList.Items {
		pod := &podList.Items[i]

		// Skip mirror pods (managed by kubelet)
		if _, isMirror := pod.Annotations["kubernetes.io/config.mirror"]; isMirror {
			skippedMirr++
			continue
		}

		// Skip DaemonSet-owned pods
		if isDaemonSetPod(pod) {
			skippedDS++
			continue
		}

		toEvict++
		wg.Add(1)
		go func() {
			defer wg.Done()

			logger.V(1).Info("evicting pod", "pod", pod.Name, "namespace", pod.Namespace, "node", nodeName)
			if err := evictPod(ctx, client, pod.Namespace, pod.Name); err != nil {
				if !apierrors.IsNotFound(err) {
					mu.Lock()
					errs = append(errs, fmt.Errorf("evicting pod %s/%s: %w", pod.Namespace, pod.Name, err))
					mu.Unlock()
				}
				return
			}

			// Wait for pod to be deleted
			if err := waitForPodDeletion(ctx, client, pod.Namespace, pod.Name); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("waiting for pod %s/%s deletion: %w", pod.Namespace, pod.Name, err))
				mu.Unlock()
			}
		}()
	}

	logger.Info("draining pods",
		"node", nodeName,
		"totalPods", len(podList.Items),
		"toEvict", toEvict,
		"skippedDaemonSet", skippedDS,
		"skippedMirror", skippedMirr,
	)

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("drain errors: %v", errs)
	}
	return nil
}

func isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func evictPod(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	for {
		err := client.PolicyV1().Evictions(namespace).Evict(ctx, eviction)
		if err == nil {
			return nil
		}
		if apierrors.IsNotFound(err) {
			return nil
		}
		if apierrors.IsTooManyRequests(err) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}
		return err
	}
}

func waitForPodDeletion(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}
