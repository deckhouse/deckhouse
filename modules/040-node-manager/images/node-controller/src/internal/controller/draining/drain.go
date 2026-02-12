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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// drainNodePods evicts all pods from a node, respecting the context timeout.
// It mirrors the behavior of the drain.RunNodeDrain helper used in the original hook
// with Force=true, IgnoreAllDaemonSets=true, DeleteEmptyDirData=true.
func drainNodePods(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	logger := log.FromContext(ctx)

	pods, err := listNodePods(ctx, client, nodeName)
	if err != nil {
		return err
	}

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		errs        []error
		skippedDS   int
		skippedMirr int
		toEvict     int
	)

	for i := range pods {
		pod := &pods[i]

		evict, reason := shouldEvictPod(ctx, client, pod)
		if !evict {
			switch reason {
			case reasonMirrorPod:
				skippedMirr++
			default:
				skippedDS++
			}
			logger.V(1).Info("skipping pod", "pod", pod.Name, "namespace", pod.Namespace, "reason", reason)
			continue
		}

		toEvict++
		wg.Add(1)
		go func(p *corev1.Pod) {
			defer wg.Done()

			logger.V(1).Info("evicting pod", "pod", p.Name, "namespace", p.Namespace, "node", nodeName)
			if err := evictPod(ctx, client, p); err != nil {
				if !apierrors.IsNotFound(err) {
					mu.Lock()
					errs = append(errs, fmt.Errorf("evicting pod %s/%s: %w", p.Namespace, p.Name, err))
					mu.Unlock()
				}
				return
			}

			if err := waitForPodDeletion(ctx, client, p.Namespace, p.Name, p.UID); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("waiting for pod %s/%s deletion: %w", p.Namespace, p.Name, err))
				mu.Unlock()
			}
		}(pod)
	}

	logger.Info("draining pods",
		"node", nodeName,
		"totalPods", len(pods),
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

// listNodePods lists all pods on a node, handling pagination via Continue tokens.
func listNodePods(ctx context.Context, client kubernetes.Interface, nodeName string) ([]corev1.Pod, error) {
	var allPods []corev1.Pod
	opts := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String(),
	}
	for {
		podList, err := client.CoreV1().Pods("").List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("listing pods on node %s: %w", nodeName, err)
		}
		allPods = append(allPods, podList.Items...)
		if podList.Continue == "" {
			break
		}
		opts.Continue = podList.Continue
	}
	return allPods, nil
}

// Pod filter reasons for logging.
const (
	reasonMirrorPod = "mirror pod"
	reasonDaemonSet = "DaemonSet-managed pod"
)

// shouldEvictPod applies the filter chain matching the original hook behavior:
// 1. daemonSetFilter (with IgnoreAllDaemonSets=true, Force=true)
// 2. mirrorPodFilter
// Filters for localStorage and unreplicated are no-ops with DeleteEmptyDirData=true and Force=true.
func shouldEvictPod(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) (bool, string) {
	// Filter 1: DaemonSet filter (enhanced)
	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef != nil && controllerRef.Kind == "DaemonSet" {
		// Finished DaemonSet pods (Succeeded/Failed) can always be evicted.
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			return true, ""
		}

		// Check if the DaemonSet still exists.
		_, err := client.AppsV1().DaemonSets(pod.Namespace).Get(ctx, controllerRef.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Orphaned DaemonSet pod — with Force=true, evict it.
				return true, ""
			}
			// Unexpected error — skip to be safe.
			return false, fmt.Sprintf("error checking DaemonSet %s/%s: %v", pod.Namespace, controllerRef.Name, err)
		}

		// DaemonSet exists, IgnoreAllDaemonSets=true — skip.
		return false, reasonDaemonSet
	}

	// Filter 2: Mirror pod filter
	if _, isMirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
		return false, reasonMirrorPod
	}

	// Filters 3-4 (localStorage, unreplicated) are no-ops with Force=true and DeleteEmptyDirData=true.
	return true, ""
}

// evictPod evicts a pod with retry logic for PDB violations and namespace termination.
// Matches the original hook's eviction retry behavior.
func evictPod(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}

	activePod := pod
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := client.PolicyV1().Evictions(activePod.Namespace).Evict(ctx, eviction)
		if err == nil {
			return nil
		}
		if apierrors.IsNotFound(err) {
			return nil
		}

		// PDB violation or rate limiting — retry after 5s.
		if apierrors.IsTooManyRequests(err) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}

		// Namespace terminating — pod may be deleted by namespace controller.
		if apierrors.IsForbidden(err) && apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
			if !activePod.DeletionTimestamp.IsZero() {
				// Pod already marked for deletion — eviction won't succeed but pod is being cleaned up.
				return nil
			}
			// Namespace terminating but pod not yet marked — refresh and retry.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			freshPod, getErr := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if getErr == nil {
				activePod = freshPod
			}
			continue
		}

		return err
	}
}

// waitForPodDeletion polls until the pod is deleted or replaced (UID changed).
func waitForPodDeletion(ctx context.Context, client kubernetes.Interface, namespace, name string, uid types.UID) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		p, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		// If UID changed, the original pod was deleted and a replacement was created.
		if p.UID != uid {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}
