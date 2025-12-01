// Copyright 2025 Flant JSC
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

package main

import (
	"context"
	"flag"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/containerd/containerd"
	containerdEvents "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl/v2"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/kmsg"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
)

var (
	isReady       atomic.Bool
	containerdOK  atomic.Bool
	lastEventHB   atomic.Int64
	lastKmsgHB    atomic.Int64
)

var (
	defaultPattern            = `^oom-kill.+,task_memcg=\/kubepods(?:\.slice)?\/.+\/(?:kubepods-burstable-)?pod(\w+[-_]\w+[-_]\w+[-_]\w+[-_]\w+)(?:\.slice)?\/(?:cri-containerd-)?([a-f0-9]+)`
	kmesgRE                   = regexp.MustCompile(defaultPattern)
	kubernetesCounterVec      *prometheus.CounterVec
	prometheusContainerLabels = map[string]string{
		"io.kubernetes.container.name": "container_name",
		"io.kubernetes.pod.namespace":  "namespace",
		"io.kubernetes.pod.uid":        "pod_uid",
		"io.kubernetes.pod.name":       "pod_name",
	}
	metricsAddr string
	newPattern  string
)

func init() {
	flag.StringVar(&metricsAddr, "listen-address", "127.0.0.1:4205", "The address to listen on for HTTP requests.")
	flag.StringVar(&newPattern, "regexp-pattern", defaultPattern, "Overwrites the default regexp pattern to match and extract Pod UID and Container ID.")
}

func main() {
	flag.Parse()

	if newPattern != "" {
		kmesgRE = regexp.MustCompile(newPattern)
	}

	containerdClient, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		glog.Fatal(err)
	}
	defer containerdClient.Close()

	var labels []string
	for _, label := range prometheusContainerLabels {
		labels = append(labels, strings.ReplaceAll(label, ".", "_"))
	}
	kubernetesCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "klog_pod_oomkill",
		Help: "Extract metrics for OOMKilled pods from kernel log",
	}, labels)

	prometheus.MustRegister(kubernetesCounterVec)

	go func() {
		glog.Info("Starting prometheus metrics")

		mux := http.NewServeMux()

		mux.Handle("/metrics", promhttp.Handler())

		mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
			if !isReady.Load() {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
		})

		mux.HandleFunc("/live", func(w http.ResponseWriter, _ *http.Request) {
			if !containerdOK.Load() {
				http.Error(w, "containerd not reachable", http.StatusServiceUnavailable)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("alive"))
		})

		server := &http.Server{
			Addr:              metricsAddr,
			ReadHeaderTimeout: 3 * time.Second,
			Handler:           mux,
		}

		glog.Warning(server.ListenAndServe())
	}()

	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")

	initialResync(ctx, containerdClient)

	go watchContainerd(ctx, containerdClient)

	kmsgWatcher := kmsg.NewKmsgWatcher(types.WatcherConfig{Plugin: "kmsg"})
	logCh, err := kmsgWatcher.Watch()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			lastKmsgHB.Store(time.Now().Unix())
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			lastEventHB.Store(time.Now().Unix())
		}
	}()

	if err != nil {
		glog.Fatal("Could not create log watcher")
	}

	for log := range logCh {
		podUID, containerID := getContainerIDFromLog(log.Message)
		if containerID != "" {
			labels, err := getContainerLabels(containerID, containerdClient)
			if err != nil || labels == nil {
				glog.Warningf("Could not get labels for container id %s, pod %s: %v", containerID, podUID, err)
			} else {
				prometheusCount(labels)
			}
		}
	}
}

func watchContainerd(ctx context.Context, cli *containerd.Client) {
	eventCh, errCh := cli.EventService().Subscribe(
		ctx,
		"topic~=^/containers/",
	)

	for {
		select {
		case e := <-eventCh:
			handleContainerdEvent(ctx, cli, e)
		case err := <-errCh:
			glog.Errorf("containerd event error: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func handleContainerdEvent(ctx context.Context, cli *containerd.Client, e *events.Envelope) {
	containerdOK.Store(true)

	switch e.Topic {
	case "/containers/create":
		obj, err := typeurl.UnmarshalAny(e.Event)
		if err != nil {
			glog.Warningf("Failed to unmarshal containerd event: %v", err)
			return
		}

		ev, ok := obj.(*containerdEvents.ContainerCreate)
		if !ok {
			glog.Warning("Unexpected event type")
			return
		}

		container, err := cli.ContainerService().Get(ctx, ev.ID)
		if err != nil {
			glog.Warningf("Failed to get container %s: %v", ev.ID, err)
			return
		}

		prometheusEnsureSeries(container.Labels)
		glog.V(4).Infof("Registered zero series for container %s", ev.ID)
	}
}

func prometheusEnsureSeries(containerLabels map[string]string) {
	labels := make(map[string]string)
	for key, label := range prometheusContainerLabels {
		labels[label] = containerLabels[key]
	}
	kubernetesCounterVec.With(labels).Add(0)
}

func prometheusCount(containerLabels map[string]string) {
	labels := make(map[string]string)
	for key, label := range prometheusContainerLabels {
		labels[label] = containerLabels[key]
	}

	counter, err := kubernetesCounterVec.GetMetricWith(labels)
	if err != nil {
		glog.Warning(err)
		return
	}

	counter.Add(1)
}

func getContainerIDFromLog(log string) (podUID, containerID string) {
	podUID = ""
	containerID = ""

	if matches := kmesgRE.FindStringSubmatch(log); matches != nil {
		podUID = matches[1]
		containerID = matches[2]
	}

	return
}

func getContainerLabels(containerID string, cli *containerd.Client) (map[string]string, error) {
	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")
	container, err := cli.ContainerService().Get(ctx, containerID)
	if err != nil {
		return nil, err
	}

	return container.Labels, nil
}

func initialResync(ctx context.Context, cli *containerd.Client) {
	containers, err := cli.ContainerService().List(ctx)
	if err != nil {
		glog.Errorf("Initial resync failed: %v", err)
		return
	}

	glog.V(4).Infof("Initial resync: found %d containers", len(containers))

	for _, c := range containers {
		if c.Labels == nil {
			continue
		}

		prometheusEnsureSeries(c.Labels)
	}
	isReady.Store(true)
}