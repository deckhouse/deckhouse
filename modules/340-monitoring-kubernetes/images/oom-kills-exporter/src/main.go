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
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/kmsg"
	"k8s.io/node-problem-detector/pkg/systemlogmonitor/logwatchers/types"
)

var (
	local  = prometheus.NewRegistry()
	sleepG time.Duration
)

func checkMetricExistenceByLabels(metricName string, labels map[string]string, r *prometheus.Registry) bool {
	mfs, err := r.Gather()
	if err != nil {
		log.Println("Error gathering metrics:", err)
		return false
	}

	for _, mf := range mfs {
		if mf.GetName() == metricName {
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
	}
	return false
}

func getContainerIDFromLog(line string) (string, error) {
	var taskMemcg string
	if matches := regexp.MustCompile(`^oom-kill:(.+)`).FindStringSubmatch(line); matches != nil {
		log.Print(line)
		for _, word := range strings.Split(matches[0], ",") {
			if strings.Contains(word, "task_memcg") {
				if idx := strings.Index(word, "="); idx != -1 {
					taskMemcg = word[idx+1:]
				}
			}
		}
		return taskMemcg, nil
	}
	return "", errors.New("Don't oom-kill log")
}

func init() {
	if envValue := os.Getenv("PROMETHEUS_SCRAPE_INTERVAL"); envValue != "" {
		interval, err := strconv.Atoi(envValue)
		if err != nil {
			log.Fatal("PROMETHEUS_SCRAPE_INTERVAL must be a number")
		}
		sleepG = time.Duration(interval+1) * time.Second
	}

}

func main() {
	handler := promhttp.HandlerFor(
		local,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})
	klogOomkill := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "klog_oomkill"},
		[]string{"task_memcg"},
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
		if taskMemcg, err := getContainerIDFromLog(item.Message); err == nil {
			labels := map[string]string{
				"task_memcg": taskMemcg,
			}
			sleep := 0 * time.Second
			if !checkMetricExistenceByLabels("klog_oomkill", labels, local) {
				// The GetMetricWith query creates a metric with 0 value.
				if _, err := klogOomkill.GetMetricWith(labels); err != nil {
					log.Fatal("Could not create metrics")
				}
				sleep = sleepG // Delay so that prometeus can read the metric value with zero value.
			}

			go func() {
				time.Sleep(sleep)
				klogOomkill.With(labels).Inc()
			}()
		}
	}
}
