// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filter

import (
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetArgFromUnstructuredPodWithRegexp(obj *unstructured.Unstructured, exp *regexp.Regexp, captureIndex int, containerName string) (string, error) {
	var pod v1.Pod
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return "", fmt.Errorf("from unstructured: %w", err)
	}

	return GetArgPodWithRegexp(&pod, exp, captureIndex, containerName), nil
}

func GetArgPodWithRegexp(pod *v1.Pod, exp *regexp.Regexp, captureIndex int, containerName string) string {
	containerIndex := 0
	if containerName != "" {
		for i, c := range pod.Spec.Containers {
			if c.Name == containerName {
				containerIndex = i
				break
			}
		}
	}

	match := parseFromArgs(exp, pod.Spec.Containers[containerIndex].Command, captureIndex)
	if match != "" {
		return match
	}

	match = parseFromArgs(exp, pod.Spec.Containers[containerIndex].Args, captureIndex)
	if match != "" {
		return match
	}

	return ""
}

func parseFromArgs(exp *regexp.Regexp, args []string, captureIndex int) string {
	for _, arg := range args {
		clusterDomainMatches := exp.FindAllStringSubmatch(arg, -1)
		if len(clusterDomainMatches) < 1 {
			continue
		}

		// 0 - index is fullmatch
		indx := captureIndex + 1

		if len(clusterDomainMatches[0]) < indx+1 {
			continue
		}

		return clusterDomainMatches[0][indx]
	}

	return ""
}
