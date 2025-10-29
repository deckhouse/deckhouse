/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"context"
	"os"
	"sort"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/app/tasks"
	"d8_shutdown_inhibitor/pkg/kubernetes"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

func ListPods(podLabel string) {
	nodeName, err := os.Hostname()
	if err != nil {
		dlog.Fatal("list pods: failed to get hostname", dlog.Err(err))
	}

	kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
	if err != nil {
		dlog.Fatal("list pods: failed to create kubernetes client", dlog.Err(err))
	}

	podObserver := &tasks.PodObserver{
		NodeName: nodeName,
		PodMatchers: []kubernetes.PodMatcher{
			kubernetes.WithLabel(podLabel),
			kubernetes.WithRunningPhase(),
		},
		Klient: kubeClient,
	}

	pods, err := podObserver.ListMatchedPods(context.Background())
	if err != nil {
		dlog.Error("list pods: failed to list matched pods", dlog.Err(err), slog.String("node", nodeName))
		return
	}

	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	dlog.Info("pods with label", slog.String("label", podLabel), slog.Int("count", len(pods)))
	for _, pod := range pods {
		dlog.Info("pod matched",
			slog.String("name", pod.Name),
			slog.String("namespace", pod.Namespace),
			slog.String("node", nodeName),
		)
	}
}
