/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

type PodMatcher func(pod Pod) bool

func WithLabel(label string) func(pod Pod) bool {
	return func(pod Pod) bool {
		if label == "" {
			return false
		}
		_, ok := pod.Metadata.Labels[label]
		return ok
	}
}

func WithRunningPhase() func(pod Pod) bool {
	return func(pod Pod) bool {
		return pod.Status != nil && pod.Status.Phase == "Running"
	}
}

func FilterPods(pods []Pod, matchers ...PodMatcher) []Pod {
	if len(matchers) == 0 {
		return pods
	}

	filtered := make([]Pod, 0)
	for _, pod := range pods {
		isMatched := true
		for _, matcher := range matchers {
			if matcher != nil && matcher(pod) {
				continue
			}
			isMatched = false
			break
		}
		if isMatched {
			filtered = append(filtered, pod)
		}
	}

	return filtered
}
