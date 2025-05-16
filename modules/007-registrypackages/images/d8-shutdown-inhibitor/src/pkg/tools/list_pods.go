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

package tools

import (
	"fmt"
	"os"
	"sort"

	"d8_shutdown_inhibitor/pkg/app/tasks"
	"d8_shutdown_inhibitor/pkg/kubernetes"
)

func ListPods(podLabel string) {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("START Error: get hostname: %v\n", err)
		os.Exit(1)
	}

	podObserver := &tasks.PodObserver{
		NodeName: nodeName,
		PodMatchers: []kubernetes.PodMatcher{
			kubernetes.WithLabel(podLabel),
			kubernetes.WithRunningPhase(),
		},
	}

	pods, err := podObserver.ListMatchedPods()
	if err != nil {
		fmt.Printf("List matched Pods: %v\n", err)
	}

	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].Metadata.Name < pods[j].Metadata.Name
	})

	fmt.Printf("Pods with label %s:\n", podLabel)
	for _, pod := range pods {
		fmt.Printf("  %s\n", pod.Metadata.Name)
	}
}
