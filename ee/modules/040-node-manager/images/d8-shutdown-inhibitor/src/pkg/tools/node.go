/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"context"
	"os"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
	"d8_shutdown_inhibitor/pkg/kubernetes"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

func NodeName() {
	nodeName, err := os.Hostname()
	if err != nil {
		dlog.Error("node tool: failed to get hostname", dlog.Err(err))
		return
	}
	dlog.Info("node name", slog.String("node", nodeName))
}

func NodeCordon() {
	nodeName, err := os.Hostname()
	if err != nil {
		dlog.Error("node cordon: failed to get hostname", dlog.Err(err))
		return
	}
	dlog.Info("node cordon: operating on node", slog.String("node", nodeName))

	kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
	if err != nil {
		dlog.Error("node cordon: failed to init kubernetes client", dlog.Err(err))
		return
	}

	ctx := context.Background()
	node := kubeClient.GetNode(ctx, nodeName).Cordon(ctx)
	if err := node.Err(); err != nil {
		dlog.Error("node cordon: failed to cordon", dlog.Err(err), slog.String("node", nodeName))
		return
	}

	if err := kubeClient.GetNode(ctx, nodeName).SetCordonAnnotation(ctx).Err(); err != nil {
		dlog.Error("node cordon: failed to set cordon annotation", dlog.Err(err), slog.String("node", nodeName))
		return
	}

	dlog.Info("node cordoned by shutdown inhibitor", slog.String("node", nodeName))
}

func NodeCondition(stage string) {
	nodeName, err := os.Hostname()
	if err != nil {
		dlog.Error("node condition: failed to get hostname", dlog.Err(err))
		return
	}
	dlog.Info("node condition: operating on node", slog.String("node", nodeName))

	kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
	if err != nil {
		dlog.Error("node condition: failed to init kubernetes client", dlog.Err(err))
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
		dlog.Error("node condition: update failed", dlog.Err(err), slog.String("node", nodeName), slog.String("stage", stage))
	}
}
