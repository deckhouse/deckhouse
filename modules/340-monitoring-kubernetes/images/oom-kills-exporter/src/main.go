/*
Copyright 2024 Flant JSC
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

package main

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/kmsg"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
)

var (
	local = prometheus.NewRegistry()
)

func getContainerIDFromLog(line string) (string, string, string, error) {
	var globalOom, oomMemcg, taskMemcg string
	globalOom = "0"
	if matches := regexp.MustCompile(`^oom-kill:(.+)`).FindStringSubmatch(line); matches != nil {
		for _, word := range strings.Split(matches[0], ",") {
			switch {
			case strings.Contains(word, "global_oom"):
				globalOom = "1"
			case strings.Contains(word, "oom_memcg"):
				if idx := strings.Index(word, "="); idx != -1 {
					oomMemcg = word[idx+1:]
				}
			case strings.Contains(word, "task_memcg"):
				if idx := strings.Index(word, "="); idx != -1 {
					taskMemcg = word[idx+1:]
				}
			}
		}
		return globalOom, oomMemcg, taskMemcg, nil
	}
	return "", "", "", errors.New("Don't oom-kill log")
}

func main() {
	handler := promhttp.HandlerFor(
		local,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})
	klogOomkill := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "klog_oomkill"},
		[]string{"global_oom", "task_memcg", "oom_memcg"},
	)
	local.MustRegister(klogOomkill)

	go func() {
		log.Print("Starting prometheus metrics")
		http.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		http.Handle("/metrics", handler)
		log.Fatal(http.ListenAndServe("127.0.0.1:4205", nil))
	}()

	kmsgWatcher := kmsg.NewKmsgWatcher(types.WatcherConfig{Plugin: "kmsg", Lookback: "240h"})
	logCh, err := kmsgWatcher.Watch()
	if err != nil {
		log.Fatal("Could not create log watcher")
	}

	for item := range logCh {
		globalOom, oomMemcg, taskMemcg, err := getContainerIDFromLog(item.Message)
		if err == nil {
			klogOomkill.WithLabelValues(globalOom, taskMemcg, oomMemcg).Add(1)
		}
	}
}
