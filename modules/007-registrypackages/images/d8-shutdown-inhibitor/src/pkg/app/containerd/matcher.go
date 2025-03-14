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

package containerd

type PodState string

const CriSandboxReady PodState = "SANDBOX_READY"

type PodMatcher func(pod Pod) bool

func WithLabel(label string) func(pod Pod) bool {
	return func(pod Pod) bool {
		if label == "" {
			return false
		}
		_, ok := pod.Labels[label]
		return ok
	}
}

func WithReadyState() func(pod Pod) bool {
	return func(pod Pod) bool {
		return pod.State == string(CriSandboxReady)
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
