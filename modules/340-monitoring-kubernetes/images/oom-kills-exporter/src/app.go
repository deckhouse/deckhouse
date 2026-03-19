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
	"flag"
	"maps"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/cache"
)

const (
	defaultPattern = `^oom-kill.+,task_memcg=\/kubepods(?:\.slice)?\/.+\/(?:kubepods-burstable-)?pod(\w+[-_]\w+[-_]\w+[-_]\w+[-_]\w+)(?:\.slice)?\/(?:cri-containerd-)?([a-f0-9]+)`
	podUIDIndex    = "podUID"
)

type app struct {
	isReady   atomic.Bool
	kubeAPIOK atomic.Bool

	kmesgRE              *regexp.Regexp
	kubernetesCounterVec *prometheus.CounterVec
	containerLabels      []string
	nodeName             string

	labelsMu            sync.RWMutex
	containerLabelsByID map[string]map[string]string

	podsMu            sync.Mutex
	containerIDsByPod map[string]map[string]struct{}
	podIndexer        cache.Indexer

	pendingCleanupMu sync.Mutex
	pendingCleanup   []prometheus.Labels

	metricsAddr string
}

func newApp() *app {
	var metricsAddr string
	var newPattern string

	flag.StringVar(&metricsAddr, "listen-address", "127.0.0.1:4205", "The address to listen on for HTTP requests.")
	flag.StringVar(&newPattern, "regexp-pattern", defaultPattern, "Overwrites the default regexp pattern to match and extract Pod UID and Container ID.")
	flag.Parse()

	a := &app{
		kmesgRE:         regexp.MustCompile(newPattern),
		metricsAddr:     metricsAddr,
		containerLabels: []string{"container_name", "namespace", "pod_name", "node_name"},
	}

	a.nodeName = os.Getenv("NODE_NAME")
	if a.nodeName == "" {
		a.nodeName = "unknown"
	}

	labels := prepareLabels(a)
	a.kubernetesCounterVec = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "klog_pod_oomkill",
		Help: "Extract metrics for OOMKilled pods from kernel log",
	}, labels)
	prometheus.MustRegister(a.kubernetesCounterVec)

	return a
}

func (a *app) prometheusEnsureSeries(containerLabels map[string]string) {
	a.kubernetesCounterVec.With(a.buildPrometheusLabels(containerLabels)).Add(0)
}

func (a *app) prometheusCount(containerLabels map[string]string) {
	labels := a.buildPrometheusLabels(containerLabels)

	counter, err := a.kubernetesCounterVec.GetMetricWith(labels)
	if err != nil {
		glog.Warningf("Could not get metric with labels %v: %v", labels, err)
		return
	}

	counter.Add(1)
}

func (a *app) getContainerIDFromLog(log string) (string, string) {
	podUID := ""
	containerID := ""

	if matches := a.kmesgRE.FindStringSubmatch(log); matches != nil {
		podUID = matches[1]
		containerID = matches[2]
	}

	return podUID, containerID
}

func (a *app) buildPrometheusLabels(containerLabels map[string]string) prometheus.Labels {
	labels := maps.Clone(containerLabels)
	labels["node_name"] = a.nodeName
	return labels
}

func (a *app) trackContainerLabels(containerID string, labels map[string]string) {
	a.labelsMu.Lock()
	defer a.labelsMu.Unlock()
	if a.containerLabelsByID == nil {
		a.containerLabelsByID = make(map[string]map[string]string)
	}
	a.containerLabelsByID[containerID] = copyLabels(labels)
}

func (a *app) getTrackedLabels(containerID string) (map[string]string, bool) {
	a.labelsMu.RLock()
	defer a.labelsMu.RUnlock()
	var labels map[string]string
	labels, ok := a.containerLabelsByID[containerID]
	return copyLabels(labels), ok
}

func (a *app) deleteTrackedLabels(containerID string) {
	a.labelsMu.Lock()
	defer a.labelsMu.Unlock()
	delete(a.containerLabelsByID, containerID)
}

func (a *app) markForCleanup(labels prometheus.Labels) {
	a.pendingCleanupMu.Lock()
	defer a.pendingCleanupMu.Unlock()
	a.pendingCleanup = append(a.pendingCleanup, labels)
}

func (a *app) flushPendingCleanup() {
	a.pendingCleanupMu.Lock()
	defer a.pendingCleanupMu.Unlock()
	for _, labels := range a.pendingCleanup {
		a.kubernetesCounterVec.Delete(labels)
	}
	a.pendingCleanup = nil
}

func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	copied := make(map[string]string, len(labels))
	for key, value := range labels {
		copied[key] = value
	}
	return copied
}

func prepareLabels(a *app) []string {
	labels := make([]string, 0, len(a.containerLabels))

	for _, label := range a.containerLabels {
		labels = append(labels, strings.ReplaceAll(label, ".", "_"))
	}

	return labels
}
