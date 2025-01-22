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

const (
	labelThesholdPrefix    = "threshold.extended-monitoring.deckhouse.io/"
	namespacesEnabledLabel = "extended-monitoring.deckhouse.io/enabled"
	intervalRecordMetrics  = 5 * time.Minute
	timeOutHealthz         = time.Duration(3 * intervalRecordMetrics)
)

var (
	options              = metav1.ListOptions{TimeoutSeconds: &timeOut}
	optionsNs            = metav1.ListOptions{LabelSelector: namespacesEnabledLabel, TimeoutSeconds: &timeOut}
	resourceNodes        = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	resourceNamespaces   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	resourcePods         = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	resourceIngresses    = schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}
	resourceDeployments  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	resourceDaemonsets   = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	resourceStatefulsets = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	resourceCronjobs     = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
)

var (
	timeHealthz      = time.Now()
	timeOut          = int64(60)
	kubeClient       *kubernetes.Clientset
	kubeMetadata     metadata.Interface
	reg              = prometheus.NewRegistry()
	nodeThresholdMap = map[string]float64{
		"disk-bytes-warning":             70,
		"disk-bytes-critical":            80,
		"disk-inodes-warning":            90,
		"disk-inodes-critical":           95,
		"load-average-per-core-warning":  3,
		"load-average-per-core-critical": 10,
	}
)

func ListResources(ctx context.Context, client metadata.Interface, resource schema.GroupVersionResource, option metav1.ListOptions, namespace string) *metav1.PartialObjectMetadataList {
	var request metadata.ResourceInterface
	if namespace != "" {
		request = client.Resource(resource).Namespace(namespace)
	} else {
		request = client.Resource(resource)
	}
	rows, err := request.List(ctx, option)
	if err != nil {
		log.Print(resource.String() + " couldn't get")
		log.Fatal(err.Error())
	}
	return rows
}

func enabledLabel(labels map[string]string) float64 {
	if value, ok := labels[namespacesEnabledLabel]; ok {
		if enabled, err := strconv.ParseBool(value); err == nil && !enabled {
			return 0
		}
	}
	return 1
}

func thresholdLabel(labels map[string]string, threshold string, i float64) float64 {
	if value, ok := labels[labelThesholdPrefix+threshold]; ok {
		if tmp, err := strconv.ParseFloat(value, 64); err != nil {
			log.Printf("[thresholdLabel] could not parse the value of \"%s\": %v\n", labelThesholdPrefix+threshold, err)
		} else {
			i = tmp
		}
	}
	return i
}

func thresholdMetric(
	ctx context.Context,
	client metadata.Interface, maps map[string]float64,
	namespasce metav1.PartialObjectMetadata,
	resource schema.GroupVersionResource,
	enabled *prometheus.CounterVec,
	threshold *prometheus.CounterVec) {
	for key, value := range maps {
		maps[key] = thresholdLabel(namespasce.Labels, key, value)
	}
	for _, pod := range ListResources(ctx, client, resource, options, namespasce.Name).Items {
		e := enabledLabel(pod.Labels)
		enabled.WithLabelValues(namespasce.Name, pod.Name).Add(e)
		if e == 1 {
			for key, value := range maps {
				threshold.WithLabelValues(namespasce.Name, pod.Name, key).Add(thresholdLabel(pod.Labels, key, value))
			}
		}
	}
}

func recordMetrics(ctx context.Context, client metadata.Interface, registry *prometheus.Registry) {
	// init
	nodeEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_enabled"},
		[]string{"node"},
	)
	nodeThreshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_threshold"},
		[]string{"node", "threshold"},
	)
	namespacesEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_enabled"},
		[]string{"namespace"},
	)
	podEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_enabled"},
		[]string{"namespace", "pod"},
	)
	podThreshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_threshold"},
		[]string{"namespace", "pod", "threshold"},
	)
	ingressEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_enabled"},
		[]string{"namespace", "ingress"},
	)
	ingressThreshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_threshold"},
		[]string{"namespace", "ingress", "threshold"},
	)
	deploymentEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_enabled"},
		[]string{"namespace", "deployment"},
	)
	deploymentThreshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_threshold"},
		[]string{"namespace", "deployment", "threshold"},
	)
	daemonsetEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_enabled"},
		[]string{"namespace", "daemonset"},
	)
	daemonsetThreshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_threshold"},
		[]string{"namespace", "daemonset", "threshold"},
	)
	statefulsetEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_enabled"},
		[]string{"namespace", "statefulset"},
	)
	statefulsetThreshold := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_threshold"},
		[]string{"namespace", "statefulset", "threshold"},
	)
	cronjobEnabled := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_cronjob_enabled"},
		[]string{"namespace", "cronjob"},
	)
	// node
	for _, node := range ListResources(ctx, client, resourceNodes, options, "").Items {
		enabled := enabledLabel(node.Labels)
		nodeEnabled.WithLabelValues(node.Name).Add(enabled)
		if enabled == 1 {
			for key, value := range nodeThresholdMap {
				nodeThreshold.WithLabelValues(node.Name, key).Add(thresholdLabel(node.Labels, key, value))
			}
		}
	}
	// namespace
	for _, namespasce := range ListResources(ctx, client, resourceNamespaces, optionsNs, "").Items {
		enabledNamespace := enabledLabel(namespasce.Labels)
		namespacesEnabled.WithLabelValues(namespasce.Name).Add(enabledNamespace)

		if enabledNamespace == 1 {
			// pod
			thresholdMetric(ctx, client, map[string]float64{
				"disk-bytes-warning":            85,
				"disk-bytes-critical":           95,
				"disk-inodes-warning":           85,
				"disk-inodes-critical":          90,
				"container-throttling-warning":  25,
				"container-throttling-critical": 50,
			}, namespasce, resourcePods, podEnabled, podThreshold)
			// ingress
			thresholdMetric(ctx, client, map[string]float64{
				"5xx-warning":  10,
				"5xx-critical": 20,
			}, namespasce, resourceIngresses, ingressEnabled, ingressThreshold)
			// deployment
			thresholdMetric(ctx, client, map[string]float64{
				"replicas-not-ready": 0,
			}, namespasce, resourceDeployments, deploymentEnabled, deploymentThreshold)
			// daemonset
			thresholdMetric(ctx, client, map[string]float64{
				"replicas-not-ready": 0,
			}, namespasce, resourceDaemonsets, daemonsetEnabled, daemonsetThreshold)
			// statefulset
			thresholdMetric(ctx, client, map[string]float64{
				"replicas-not-ready": 0,
			}, namespasce, resourceStatefulsets, statefulsetEnabled, statefulsetThreshold)
			// cronjob (one cronjobEnabled)
			thresholdMetric(ctx, client, map[string]float64{}, namespasce, resourceCronjobs, cronjobEnabled, cronjobEnabled)
		}
	}
	registry.MustRegister(nodeEnabled)
	registry.MustRegister(nodeThreshold)
	registry.MustRegister(namespacesEnabled)
	registry.MustRegister(podEnabled)
	registry.MustRegister(podThreshold)
	registry.MustRegister(ingressEnabled)
	registry.MustRegister(ingressThreshold)
	registry.MustRegister(deploymentEnabled)
	registry.MustRegister(deploymentThreshold)
	registry.MustRegister(daemonsetEnabled)
	registry.MustRegister(daemonsetThreshold)
	registry.MustRegister(statefulsetEnabled)
	registry.MustRegister(statefulsetThreshold)
	registry.MustRegister(cronjobEnabled)
}

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
			// local := prometheus.NewRegistry()
			// recordMetrics(context.Background(), kubeMetadata, local)
			// *reg = *local
			recordMetrics(context.Background(), kubeMetadata, reg)
			timeHealthz = time.Now()
			time.Sleep(intervalRecordMetrics)
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
		if time.Since(timeHealthz) > timeOutHealthz {
			log.Printf("Fail if metrics were last collected more than %v", timeOutHealthz)
			http.Error(w, "Error", http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
