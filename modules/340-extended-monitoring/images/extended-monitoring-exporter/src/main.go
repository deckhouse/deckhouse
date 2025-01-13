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
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

func ListResources(ctx context.Context, client metadata.Interface, resource schema.GroupVersionResource, option metav1.ListOptions, namespace string) *metav1.PartialObjectMetadataList {
	request := client.Resource(resource)
	if namespace != "" {
		request.Namespace(namespace)
	}
	rows, err := request.List(ctx, option)
	if err != nil {
		log.Print(resource.String() + " couldn't get")
		log.Fatal(err.Error())
	}
	return rows
}

func enabledLabel(labels map[string]string) float64 {
	var i float64 = 1
	for key, value := range labels {
		if key == namespaces_enabled_label {
			if value == "false" {
				i = 0
			}
		}
	}
	return i
}

func thresholdLabel(labels map[string]string, threshold string, i float64) float64 {
	for key, value := range labels {
		if key == (label_theshold_prefix + threshold) {
			tmp, err := strconv.ParseFloat(value, 64)
			if err != nil {
				log.Printf("[thresholdLabel] failed ParseFloat: %v\n", err)
			} else {
				i = tmp
			}
		}
	}
	return i
}

func recordMetrics(ctx context.Context, client metadata.Interface) {
	//init
	local := prometheus.NewRegistry()
	node_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_enabled"},
		[]string{"node"},
	)
	node_threshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_threshold"},
		[]string{"node", "threshold"},
	)
	namespaces_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_enabled"},
		[]string{"namespace"},
	)
	pod_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_enabled"},
		[]string{"namespace", "pod"},
	)
	pod_threshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_threshold"},
		[]string{"namespace", "pod", "threshold"},
	)
	ingress_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_enabled"},
		[]string{"namespace", "ingress"},
	)
	ingress_threshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_threshold"},
		[]string{"namespace", "ingress", "threshold"},
	)
	deployment_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_enabled"},
		[]string{"namespace", "deployment"},
	)
	deployment_threshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_threshold"},
		[]string{"namespace", "deployment", "threshold"},
	)
	daemonset_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_enabled"},
		[]string{"namespace", "daemonset"},
	)
	daemonset_threshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_threshold"},
		[]string{"namespace", "daemonset", "threshold"},
	)
	statefulset_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_enabled"},
		[]string{"namespace", "statefulset"},
	)
	statefulset_threshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_threshold"},
		[]string{"namespace", "statefulset", "threshold"},
	)
	cronjob_enabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_cronjob_enabled"},
		[]string{"namespace", "cronjob"},
	)
	//node
	for _, node := range ListResources(ctx, client, resource_nodes, options, "").Items {
		enabled := enabledLabel(node.Labels)
		node_enabled.WithLabelValues(node.Name).Add(enabled)
		if enabled == 1 {
			for key, value := range node_threshold_map {
				node_threshold.WithLabelValues(node.Name, key).Add(thresholdLabel(node.Labels, key, value))
			}
		}
	}
	//namespace
	for _, namespasce := range ListResources(ctx, client, resource_namespaces, options_ns, "").Items {
		enabled_namespace := enabledLabel(namespasce.Labels)
		namespaces_enabled.WithLabelValues(namespasce.Name).Add(enabled_namespace)

		if enabled_namespace == 1 {
			//pod
			for _, pod := range ListResources(ctx, client, resource_pods, options, namespasce.Name).Items {
				enabled := enabledLabel(pod.Labels)
				pod_enabled.WithLabelValues(namespasce.Name, pod.Name).Add(enabled)
				if enabled == 1 {
					for key, value := range pod_threshold_map {
						pod_threshold.WithLabelValues(namespasce.Name, pod.Name, key).Add(thresholdLabel(pod.Labels, key, value))
					}
				}
			}
			//ingress
			for _, ingress := range ListResources(ctx, client, resource_ingresses, options, namespasce.Name).Items {
				enabled := enabledLabel(ingress.Labels)
				ingress_enabled.WithLabelValues(namespasce.Name, ingress.Name).Add(enabled)
				if enabled == 1 {
					for key, value := range ingress_threshold_map {
						ingress_threshold.WithLabelValues(namespasce.Name, ingress.Name, key).Add(thresholdLabel(ingress.Labels, key, value))
					}
				}
			}
			//deployment
			for _, deployment := range ListResources(ctx, client, resource_deployments, options, namespasce.Name).Items {
				enabled := enabledLabel(deployment.Labels)
				deployment_enabled.WithLabelValues(namespasce.Name, deployment.Name).Add(enabled)
				if enabled == 1 {
					for key, value := range deployment_threshold_map {
						deployment_threshold.WithLabelValues(namespasce.Name, deployment.Name, key).Add(thresholdLabel(deployment.Labels, key, value))
					}
				}
			}
			//daemonset
			for _, daemonset := range ListResources(ctx, client, resource_daemonsets, options, namespasce.Name).Items {
				enabled := enabledLabel(daemonset.Labels)
				daemonset_enabled.WithLabelValues(namespasce.Name, daemonset.Name).Add(enabled)
				if enabled == 1 {
					for key, value := range daemonset_threshold_map {
						daemonset_threshold.WithLabelValues(namespasce.Name, daemonset.Name, key).Add(thresholdLabel(daemonset.Labels, key, value))
					}
				}
			}
			//statefulset
			for _, statefulset := range ListResources(ctx, client, resource_statefulsets, options, namespasce.Name).Items {
				enabled := enabledLabel(statefulset.Labels)
				statefulset_enabled.WithLabelValues(namespasce.Name, statefulset.Name).Add(enabled)
				if enabled == 1 {
					for key, value := range daemonset_threshold_map {
						statefulset_threshold.WithLabelValues(namespasce.Name, statefulset.Name, key).Add(thresholdLabel(statefulset.Labels, key, value))
					}
				}
			}
			//cronjob
			for _, cronjob := range ListResources(ctx, client, resource_cronjobs, options, namespasce.Name).Items {
				cronjob_enabled.WithLabelValues(namespasce.Name, cronjob.Name).Add(enabledLabel(cronjob.Labels))
			}
		}
	}
	local.MustRegister(node_enabled)
	local.MustRegister(node_threshold)
	local.MustRegister(namespaces_enabled)
	local.MustRegister(pod_enabled)
	local.MustRegister(pod_threshold)
	local.MustRegister(ingress_enabled)
	local.MustRegister(ingress_threshold)
	local.MustRegister(deployment_enabled)
	local.MustRegister(deployment_threshold)
	local.MustRegister(daemonset_enabled)
	local.MustRegister(daemonset_threshold)
	local.MustRegister(statefulset_enabled)
	local.MustRegister(statefulset_threshold)
	local.MustRegister(cronjob_enabled)
	*reg = *local
}

const (
	label_theshold_prefix    = "threshold.extended-monitoring.deckhouse.io/"
	namespaces_enabled_label = "extended-monitoring.deckhouse.io/enabled"
	interval_recordMetrics   = 5 * time.Minute
	timeOut_healthz          = time.Duration(3 * interval_recordMetrics)
)

var (
	options               = metav1.ListOptions{TimeoutSeconds: &timeOut}
	options_ns            = metav1.ListOptions{LabelSelector: namespaces_enabled_label, TimeoutSeconds: &timeOut}
	resource_nodes        = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	resource_namespaces   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	resource_pods         = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	resource_ingresses    = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}
	resource_deployments  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	resource_daemonsets   = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	resource_statefulsets = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	resource_cronjobs     = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
)

var (
	time_healthz       = time.Now()
	timeOut            = int64(60)
	kubeClient         *kubernetes.Clientset
	kubeMetadata       metadata.Interface
	reg                = prometheus.NewRegistry()
	ctx                = context.Background()
	node_threshold_map = map[string]float64{
		"disk-bytes-warning":             70,
		"disk-bytes-critical":            80,
		"disk-inodes-warning":            90,
		"disk-inodes-critical":           95,
		"load-average-per-core-warning":  3,
		"load-average-per-core-critical": 10,
	}
	pod_threshold_map = map[string]float64{
		"disk-bytes-warning":            85,
		"disk-bytes-critical":           95,
		"disk-inodes-warning":           85,
		"disk-inodes-critical":          90,
		"container-throttling-warning":  25,
		"container-throttling-critical": 50,
	}
	ingress_threshold_map = map[string]float64{
		"5xx-warning":  10,
		"5xx-critical": 20,
	}
	deployment_threshold_map = map[string]float64{
		"replicas-not-ready": 0,
	}
	daemonset_threshold_map = map[string]float64{
		"replicas-not-ready": 0,
	}
	statefulset_threshold_map = map[string]float64{
		"replicas-not-ready": 0,
	}
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error kubernetes config: %v\n", err)
	}
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error getting kubernetes config: %v\n", err)
	}
	kubeMetadata, err = metadata.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	handler := promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})
	go func() {
		for {
			recordMetrics(ctx, kubeMetadata)
			time_healthz = time.Now()
			time.Sleep(interval_recordMetrics)
		}
	}()

	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		_, err := kubeClient.ServerVersion()
		if err != nil {
			http.Error(w, "Error", http.StatusInternalServerError)
			log.Print(err)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		time_check := time.Now()
		if time_check.Sub(time_healthz) > timeOut_healthz {
			log.Printf("Fail if metrics were last collected more than %v", timeOut_healthz)
			http.Error(w, "Error", http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
