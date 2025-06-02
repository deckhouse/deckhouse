/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"fmt"
	"os"

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
	"d8_shutdown_inhibitor/pkg/kubernetes"
)

func NodeName() {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("get hostname: %v\n", err)
	}
	fmt.Printf("node name: %s\n", nodeName)
}

func NodeCordon() {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("get hostname: %v\n", err)
		return
	}
	fmt.Printf("node name: %s\n", nodeName)

	kubectl := kubernetes.NewDefaultKubectl()
	output, err := kubectl.Cordon(nodeName)
	if err != nil {
		fmt.Printf("nodeCordoner: fail to cordon node: %v\n, output: %s\n", err, string(output))
	}
	fmt.Println(string(output))
}

func NodeCondition(stage string) {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("get hostname: %v\n", err)
		return
	}
	fmt.Printf("node name: %s\n", nodeName)

	switch stage {
	case "start":
		err = nodecondition.GracefulShutdownPostpone().SetOnStart(nodeName)
	case "unlock":
		err = nodecondition.GracefulShutdownPostpone().UnsetOnUnlock(nodeName)
	case "pods":
		err = nodecondition.GracefulShutdownPostpone().SetPodsArePresent(nodeName)
	}

	if err != nil {
		fmt.Printf("update condition: %v\n", err)
	}
}
