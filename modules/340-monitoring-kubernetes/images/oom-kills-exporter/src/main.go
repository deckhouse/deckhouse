/*
Copyright 2025 Flant JSC
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
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/kmsg"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
)

func checkMetricExistenceByLabels(metricName string, labels map[string]string, r *prometheus.Registry) bool {
	mfs, err := r.Gather()
	if err != nil {
		log.Println("Error gathering metrics:", err)
		return false
	}

	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, metric := range mf.Metric {
			labelMap := make(map[string]string)
			for _, label := range metric.Label {
				labelMap[*label.Name] = *label.Value
			}
			match := true
			for key, value := range labels {
				if labelMap[key] != value {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func getContainerIDFromLog(line string) string {
	match := strings.Split(line, "oom-kill:")
	if len(match) != 2 {
		return ""
	}
	log.Print(line)
	// var taskMemcg string
	for _, word := range strings.Split(match[1], ",") {
		key, value, ok := strings.Cut(word, "=")
		if key != "task_memcg" || !ok {
			continue
		}
		return value
	}
	println("[WARNING] Parse err line: " + line)
	return ""
}

func dmesgWatcher(sleepG time.Duration, registry *prometheus.Registry, counterVec *prometheus.CounterVec) {
	kmsgWatcher := kmsg.NewKmsgWatcher(types.WatcherConfig{Plugin: "kmsg", Lookback: "240h"})
	defer kmsgWatcher.Stop()
	logCh, err := kmsgWatcher.Watch()
	if err != nil {
		log.Fatal("Could not create log watcher")
	}

	for item := range logCh {
		if taskMemcg := getContainerIDFromLog(item.Message); taskMemcg != "" {
			labels := map[string]string{
				"task_memcg": taskMemcg,
			}
			sleep := 0 * time.Second
			if !checkMetricExistenceByLabels("klog_oomkill", labels, registry) {
				// The GetMetricWith query creates a metric with 0 value.
				if _, err := counterVec.GetMetricWith(labels); err != nil {
					log.Fatal("Could not create metrics")
				}
				sleep = sleepG // Delay so that prometeus can read the metric value with zero value.
			}

			go func() {
				time.Sleep(sleep)
				counterVec.With(labels).Inc()
			}()
		}
	}
}

func main() {
	var sleepG time.Duration
	if envValue := os.Getenv("PROMETHEUS_SCRAPE_INTERVAL"); envValue != "" {
		interval, err := strconv.Atoi(envValue)
		if err != nil {
			log.Fatal("PROMETHEUS_SCRAPE_INTERVAL must be a number")
		}
		sleepG = time.Duration(interval+1) * time.Second
	}
	registry := prometheus.NewRegistry()
	handler := promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})
	klogOomkill := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "klog_oomkill"},
		[]string{"task_memcg"},
	)
	if err := registry.Register(klogOomkill); err != nil {
		log.Fatal(err.Error())
	}

	go func() {
		dmesgWatcher(sleepG, registry, klogOomkill)
	}()

	log.Print("Starting prometheus metrics")
	http.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe("127.0.0.1:4205", nil))
}
