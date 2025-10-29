/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

type PodMatcher func(pod Pod) bool

func WithLabel(label string) func(pod Pod) bool {
	return func(pod Pod) bool {
		if label == "" {
			return false
		}
		if pod.Labels == nil {
			return false
		}
		_, ok := pod.Labels[label]
		return ok
	}
}

func WithRunningPhase() func(pod Pod) bool {
	return func(pod Pod) bool {
		return pod.Status.Phase == corev1.PodRunning
	}
}

func (c *Klient) FilterPods(podList *PodList, matchers ...PodMatcher) []corev1.Pod {
	if len(matchers) == 0 {
		out := make([]corev1.Pod, len(podList.Items))
		for i := range podList.Items {
			out[i] = *podList.Items[i].DeepCopy()
		}
		return out
	}

	filtered := make([]corev1.Pod, 0, len(podList.Items))
	
	for i := range podList.Items {
		pod := podList.Items[i]
		matched := true
		for _, matcher := range matchers {
			if matcher == nil {
				continue
			}
			if !matcher(pod) {
				matched = false
				break
			}
		}
		if matched {
			filtered = append(filtered, *pod.DeepCopy())
		}
	}
	return filtered
}
