/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"context"
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

	kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
	if err != nil {
		fmt.Printf("nodeCordoner: fail to init kubernetes client: %v\n", err)
		return
	}

	ctx := context.Background()
	nodeRef := kubeClient.GetNode(ctx, nodeName).Cordon(ctx)
	if err := nodeRef.Err(); err != nil {
		fmt.Printf("nodeCordoner: fail to cordon node: %v\n", err)
		return
	}

	if err := kubeClient.GetNode(ctx, nodeName).SetCordonAnnotation(ctx).Err(); err != nil {
		fmt.Printf("nodeCordoner: fail set cordon annotation: %v\n", err)
		return
	}

	fmt.Println("node cordoned by shutdown inhibitor")
}

func NodeCondition(stage string) {
	nodeName, err := os.Hostname()
	if err != nil {
		fmt.Printf("get hostname: %v\n", err)
		return
	}
	fmt.Printf("node name: %s\n", nodeName)

	kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
	if err != nil {
		fmt.Printf("init kubernetes client: %v\n", err)
		return
	}

	ctx := context.Background()
	nc := nodecondition.GracefulShutdownPostpone(kubeClient)

	switch stage {
	case "start":
		err = nc.SetOnStart(ctx, nodeName)
	case "unlock":
		err = nc.UnsetOnUnlock(ctx, nodeName)
	case "pods":
		err = nc.SetPodsArePresent(ctx, nodeName)
	}

	if err != nil {
		fmt.Printf("update condition: %v\n", err)
	}
}
