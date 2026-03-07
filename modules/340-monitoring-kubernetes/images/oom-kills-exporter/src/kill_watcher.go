package main

import (
	"context"

	"github.com/golang/glog"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/kmsg"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
	smtypes "k8s.io/node-problem-detector/pkg/systemlogmonitor/types"
)

const maxConcurrentHandlers = 10

func (a *app) startKmsgWatcher(ctx context.Context) {
	kmsgWatcher := kmsg.NewKmsgWatcher(types.WatcherConfig{Plugin: "kmsg"})
	logCh, err := kmsgWatcher.Watch()

	if err != nil {
		glog.Fatalf("Could not create log watcher: %v", err)
	}

	sem := make(chan struct{}, maxConcurrentHandlers)

	for {
		select {
		case <-ctx.Done():
			kmsgWatcher.Stop()
			return
		case log := <-logCh:
			withWorker(log, sem, a.handleKmsgLog)
		}
	}
}

func withWorker(log *smtypes.Log, sem chan struct{}, fn func(log *smtypes.Log)) {
	if log == nil {
		return
	}
	sem <- struct{}{}

	go func(l *smtypes.Log) {
		defer func() { <-sem }()
		fn(l)
	}(log)
}

func (a *app) handleKmsgLog(log *smtypes.Log) {
	if log == nil {
		return
	}

	podUID, containerID := a.getContainerIDFromLog(log.Message)
	if containerID != "" {
		containerID = normalizeContainerID(containerID)
		if labels, ok := a.getTrackedLabels(containerID); ok {
			a.prometheusCount(labels)
			return
		}

		labels := a.getLabelsFromPodUID(podUID, containerID)
		if labels == nil {
			glog.Warningf("Could not get labels for container id %s, pod %s", containerID, podUID)
			return
		}

		a.trackContainerLabels(containerID, labels)
		a.prometheusCount(labels)
	}

}
