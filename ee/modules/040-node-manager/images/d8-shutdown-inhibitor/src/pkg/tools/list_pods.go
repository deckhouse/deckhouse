/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"fmt"
	"os"
	"os/exec"
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
		if ee, ok := err.(*exec.ExitError); ok {
			fmt.Printf("Stderr: %v\n", string(ee.Stderr))
		}
	}

	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].Metadata.Name < pods[j].Metadata.Name
	})

	fmt.Printf("Pods with label %s:\n", podLabel)
	for _, pod := range pods {
		fmt.Printf("  %s\n", pod.Metadata.Name)
	}
}
