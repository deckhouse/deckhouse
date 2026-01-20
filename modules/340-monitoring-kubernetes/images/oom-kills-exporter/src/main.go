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
	"os"
	"regexp"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/kmsg"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
)

func main() {
	var metricsAddr string
	var newPattern string
	flag.StringVar(&metricsAddr, "listen-address", "127.0.0.1:4205", "The address to listen on for HTTP requests.")
	flag.StringVar(&newPattern, "regexp-pattern", defaultPattern, "Overwrites the default regexp pattern to match and extract Pod UID and Container ID.")
	flag.Parse()

	a := &app{
		kmesgRE: regexp.MustCompile(defaultPattern),
		containerLabels: map[string]string{
			"io.kubernetes.container.name": "container_name",
			"io.kubernetes.pod.namespace":  "namespace",
			"io.kubernetes.pod.uid":        "pod_uid",
			"io.kubernetes.pod.name":       "pod_name",
		},
	}

	a.nodeName = os.Getenv("NODE_NAME")
	if a.nodeName == "" {
		a.nodeName = "unknown"
	}

	if newPattern != "" {
		a.kmesgRE = regexp.MustCompile(newPattern)
	}

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		glog.Fatal(err)
	}

	var labels []string
	for _, label := range a.containerLabels {
		labels = append(labels, strings.ReplaceAll(label, ".", "_"))
	}
	labels = append(labels, "node_name")
	a.kubernetesCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "klog_pod_oomkill",
		Help: "Extract metrics for OOMKilled pods from kernel log",
	}, labels)

	prometheus.MustRegister(a.kubernetesCounterVec)

	go a.startMetricsServer(metricsAddr)

	ctx := context.Background()

	podInformer := a.startPodWatcher(ctx, clientset)
	a.podIndexer = podInformer.GetIndexer()

	kmsgWatcher := kmsg.NewKmsgWatcher(types.WatcherConfig{Plugin: "kmsg"})
	logCh, err := kmsgWatcher.Watch()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.lastKmsgHB.Store(time.Now().Unix())
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.lastEventHB.Store(time.Now().Unix())
		}
	}()

	if err != nil {
		glog.Fatal("Could not create log watcher")
	}

	for log := range logCh {
		podUID, containerID := a.getContainerIDFromLog(log.Message)
		if containerID != "" {
			containerID = normalizeContainerID(containerID)
			if labels, ok := a.getTrackedLabels(containerID); ok {
				a.prometheusCount(labels)
				continue
			}

			labels := a.getLabelsFromPodUID(podUID, containerID)
			if labels == nil {
				glog.Warningf("Could not get labels for container id %s, pod %s", containerID, podUID)
				continue
			}

			a.trackContainerLabels(containerID, labels)
			a.prometheusCount(labels)
		}
	}
}
