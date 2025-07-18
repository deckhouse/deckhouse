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
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultListenAddress  = ":9876"
	defaultScrapeInterval = "30s"
	defaultKernelLogPath  = "/var/log/messages"
	defaultKmsgPath       = "/dev/kmsg"
)

var (
	oomKillsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oom_kills_total",
			Help: "Total number of OOM kills by namespace, pod, container and node",
		},
		[]string{"namespace", "pod_name", "container_name", "node", "uid"},
	)

	exporterInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "oom_kills_exporter_info",
			Help: "Information about the OOM kills exporter",
		},
		[]string{"version"},
	)

	processedEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "oom_kills_exporter_processed_events_total",
			Help: "Total number of processed OOM events",
		},
	)

	lastEventTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "oom_kills_exporter_last_event_timestamp_seconds",
			Help: "Timestamp of the last processed OOM event",
		},
	)

	// Регулярные выражения для парсинга OOM сообщений
	oomKillRegex1   = regexp.MustCompile(`Killed process (\d+) \(([^)]+)\)`)
	oomKillRegex2   = regexp.MustCompile(`oom-kill:.*task=([^,]+),pid=(\d+)`)
	cgroupRegex     = regexp.MustCompile(`oom_memcg=(/kubepods[^,]*)`)
	taskCgroupRegex = regexp.MustCompile(`task_memcg=(/kubepods[^,]*)`)
)

type EventKey struct {
	PID         string
	ProcessName string
	Timestamp   time.Time
}

type OOMEvent struct {
	PID           string
	ProcessName   string
	Namespace     string
	PodName       string
	ContainerName string
	UID           string
	Timestamp     time.Time
}

type OOMExporter struct {
	client         kubernetes.Interface
	seenEvents     map[EventKey]struct{}
	mutex          sync.RWMutex
	scrapeInterval time.Duration
	kernelLogPath  string
	kmsgPath       string
	nodeName       string
}

func init() {
	prometheus.MustRegister(oomKillsTotal)
	prometheus.MustRegister(exporterInfo)
	prometheus.MustRegister(processedEvents)
	prometheus.MustRegister(lastEventTimestamp)

	exporterInfo.WithLabelValues("1.0.0").Set(1)
}

func main() {
	listenAddress := getEnvOrDefault("LISTEN_ADDRESS", defaultListenAddress)
	scrapeIntervalStr := getEnvOrDefault("SCRAPE_INTERVAL", defaultScrapeInterval)
	kernelLogPath := getEnvOrDefault("KERNEL_LOG_PATH", defaultKernelLogPath)
	kmsgPath := getEnvOrDefault("KMSG_PATH", defaultKmsgPath)

	scrapeInterval, err := time.ParseDuration(scrapeIntervalStr)
	if err != nil {
		log.Fatalf("Invalid SCRAPE_INTERVAL: %v", err)
	}

	// Get node name from environment or hostname
	nodeName := getEnvOrDefault("NODE_NAME", "")
	if nodeName == "" {
		if hostname, err := os.Hostname(); err == nil {
			nodeName = hostname
		} else {
			nodeName = "unknown"
		}
	}

	// Create in-cluster config for Kubernetes API (to get pod info)
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Warning: Failed to create in-cluster config: %v", err)
	}

	var clientset kubernetes.Interface
	if config != nil {
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Printf("Warning: Failed to create kubernetes client: %v", err)
		}
	}

	exporter := &OOMExporter{
		client:         clientset,
		seenEvents:     make(map[EventKey]struct{}),
		scrapeInterval: scrapeInterval,
		kernelLogPath:  kernelLogPath,
		kmsgPath:       kmsgPath,
		nodeName:       nodeName,
	}

	// Start metrics server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	go func() {
		log.Printf("Starting metrics server on %s", listenAddress)
		if err := http.ListenAndServe(listenAddress, nil); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()

	// Start OOM event watcher
	log.Printf("Starting OOM events collector with scrape interval %s, node %s", scrapeInterval, nodeName)
	exporter.Run()
}

func (e *OOMExporter) Run() {
	ticker := time.NewTicker(e.scrapeInterval)
	defer ticker.Stop()

	// Initial scan
	e.collectOOMEvents()

	for {
		select {
		case <-ticker.C:
			e.collectOOMEvents()
		}
	}
}

func (e *OOMExporter) collectOOMEvents() {
	// Try to read from kmsg first (more reliable for recent events)
	events := e.readFromKmsg()

	// If kmsg is not available or empty, try kernel log
	if len(events) == 0 {
		events = e.readFromKernelLog()
	}

	newEvents := 0
	for _, event := range events {
		if e.processOOMEvent(event) {
			newEvents++
		}
	}

	if newEvents > 0 {
		log.Printf("Processed %d new OOM events", newEvents)
	}
}

func (e *OOMExporter) readFromKmsg() []OOMEvent {
	var events []OOMEvent

	file, err := os.Open(e.kmsgPath)
	if err != nil {
		log.Printf("Could not open %s: %v", e.kmsgPath, err)
		return events
	}
	defer file.Close()

	// Read recent messages (last 1000 lines)
	lines := make([]string, 0, 1000)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "oom-kill") || strings.Contains(line, "Killed process") {
			lines = append(lines, line)
			if len(lines) > 1000 {
				lines = lines[1:]
			}
		}
	}

	// Parse recent OOM messages
	cutoff := time.Now().Add(-2 * e.scrapeInterval)
	for _, line := range lines {
		if event := e.parseOOMLine(line); event != nil && event.Timestamp.After(cutoff) {
			events = append(events, *event)
		}
	}

	return events
}

func (e *OOMExporter) readFromKernelLog() []OOMEvent {
	var events []OOMEvent

	file, err := os.Open(e.kernelLogPath)
	if err != nil {
		log.Printf("Could not open %s: %v", e.kernelLogPath, err)
		return events
	}
	defer file.Close()

	cutoff := time.Now().Add(-2 * e.scrapeInterval)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "oom-kill") || strings.Contains(line, "Killed process") {
			if event := e.parseOOMLine(line); event != nil && event.Timestamp.After(cutoff) {
				events = append(events, *event)
			}
		}
	}

	return events
}

func (e *OOMExporter) parseOOMLine(line string) *OOMEvent {
	// Parse different OOM message formats
	var event *OOMEvent

	// Format 1: "Killed process 12345 (process-name)"
	if matches := oomKillRegex1.FindStringSubmatch(line); len(matches) >= 3 {
		event = &OOMEvent{
			PID:         matches[1],
			ProcessName: matches[2],
			Timestamp:   e.extractTimestamp(line),
		}
	}

	// Format 2: "oom-kill:...task=process-name,pid=12345"
	if matches := oomKillRegex2.FindStringSubmatch(line); len(matches) >= 3 {
		event = &OOMEvent{
			ProcessName: matches[1],
			PID:         matches[2],
			Timestamp:   e.extractTimestamp(line),
		}
	}

	if event == nil {
		return nil
	}

	// Extract cgroup information to get pod details
	if matches := cgroupRegex.FindStringSubmatch(line); len(matches) >= 2 {
		e.extractPodInfoFromCgroup(event, matches[1])
	} else if matches := taskCgroupRegex.FindStringSubmatch(line); len(matches) >= 2 {
		e.extractPodInfoFromCgroup(event, matches[1])
	}

	// If we couldn't get pod info from cgroup, try to get it from Kubernetes API
	if event.PodName == "" && e.client != nil {
		e.enrichEventWithKubernetesInfo(event)
	}

	// Set defaults if still empty
	if event.Namespace == "" {
		event.Namespace = "unknown"
	}
	if event.PodName == "" {
		event.PodName = "unknown"
	}
	if event.ContainerName == "" {
		event.ContainerName = event.ProcessName
	}
	if event.UID == "" {
		event.UID = "unknown"
	}

	return event
}

func (e *OOMExporter) extractTimestamp(line string) time.Time {
	// Try to extract timestamp from log line
	// Format examples:
	// "Jan 15 10:30:45 hostname kernel: ..."
	// "2024-01-15T10:30:45.123456+00:00 hostname kernel: ..."

	now := time.Now()

	// Simple heuristic: if line starts with month name, parse syslog format
	if strings.Contains(line[:20], "Jan") || strings.Contains(line[:20], "Feb") ||
		strings.Contains(line[:20], "Mar") || strings.Contains(line[:20], "Apr") ||
		strings.Contains(line[:20], "May") || strings.Contains(line[:20], "Jun") ||
		strings.Contains(line[:20], "Jul") || strings.Contains(line[:20], "Aug") ||
		strings.Contains(line[:20], "Sep") || strings.Contains(line[:20], "Oct") ||
		strings.Contains(line[:20], "Nov") || strings.Contains(line[:20], "Dec") {

		// Parse syslog timestamp (assume current year)
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			timeStr := fmt.Sprintf("%d %s %s", now.Year(), parts[0], parts[1], parts[2])
			if t, err := time.Parse("2006 Jan 2 15:04:05", timeStr); err == nil {
				return t
			}
		}
	}

	// If parsing fails, return current time
	return now
}

func (e *OOMExporter) extractPodInfoFromCgroup(event *OOMEvent, cgroupPath string) {
	// Parse cgroup path like:
	// /kubepods/burstable/pod123e4567-e89b-12d3-a456-426614174000/containerhash
	// /kubepods/besteffort/pod123e4567-e89b-12d3-a456-426614174000

	parts := strings.Split(cgroupPath, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "pod") {
			// Extract pod UID
			podUID := strings.TrimPrefix(part, "pod")
			podUID = strings.ReplaceAll(podUID, "_", "-")
			event.UID = podUID

			// Try to get pod info from Kubernetes API
			if e.client != nil {
				e.getPodInfoByUID(event, podUID)
			}
			break
		}
	}
}

func (e *OOMExporter) enrichEventWithKubernetesInfo(event *OOMEvent) {
	if e.client == nil {
		return
	}

	ctx := context.Background()

	// Try to find pod by process name or other heuristics
	pods, err := e.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", e.nodeName),
	})
	if err != nil {
		log.Printf("Failed to list pods: %v", err)
		return
	}

	// Simple heuristic: match by process name
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			if strings.Contains(container.Image, event.ProcessName) ||
				strings.Contains(container.Name, event.ProcessName) {
				event.Namespace = pod.Namespace
				event.PodName = pod.Name
				event.ContainerName = container.Name
				event.UID = string(pod.UID)
				return
			}
		}
	}
}

func (e *OOMExporter) getPodInfoByUID(event *OOMEvent, podUID string) {
	if e.client == nil {
		return
	}

	ctx := context.Background()

	// Search for pod by UID
	pods, err := e.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Failed to list pods: %v", err)
		return
	}

	for _, pod := range pods.Items {
		if string(pod.UID) == podUID || strings.Contains(string(pod.UID), podUID) {
			event.Namespace = pod.Namespace
			event.PodName = pod.Name
			event.UID = string(pod.UID)

			// Try to match container by process name
			for _, container := range pod.Spec.Containers {
				if strings.Contains(container.Name, event.ProcessName) ||
					strings.Contains(container.Image, event.ProcessName) {
					event.ContainerName = container.Name
					break
				}
			}
			if event.ContainerName == "" && len(pod.Spec.Containers) > 0 {
				event.ContainerName = pod.Spec.Containers[0].Name
			}
			return
		}
	}
}

func (e *OOMExporter) processOOMEvent(event OOMEvent) bool {
	eventKey := EventKey{
		PID:         event.PID,
		ProcessName: event.ProcessName,
		Timestamp:   event.Timestamp,
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Check if we've already seen this event
	if _, exists := e.seenEvents[eventKey]; exists {
		return false
	}

	// Mark event as seen
	e.seenEvents[eventKey] = struct{}{}

	// Increment the metric
	oomKillsTotal.WithLabelValues(
		event.Namespace,
		event.PodName,
		event.ContainerName,
		e.nodeName,
		event.UID,
	).Inc()

	processedEvents.Inc()
	lastEventTimestamp.Set(float64(event.Timestamp.Unix()))

	log.Printf("Recorded OOM kill: namespace=%s, pod=%s, container=%s, node=%s, pid=%s, process=%s",
		event.Namespace, event.PodName, event.ContainerName, e.nodeName, event.PID, event.ProcessName)

	return true
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
