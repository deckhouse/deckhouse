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
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	core "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func ListIngress(client kubernetes.Interface, namespasce string) (*networking.IngressList, error) {
	rows, err := client.NetworkingV1().Ingresses(namespasce).List(
		context.Background(),
		metav1.ListOptions{
			TimeoutSeconds: &timeOut,
		},
	)
	if err != nil {
		log.Print("[ingress] couldn't get")
		log.Fatal(err.Error())
	}
	return rows, nil
}

func ListPods(client kubernetes.Interface, namespasce string) (*core.PodList, error) {
	rows, err := client.CoreV1().Pods(namespasce).List(
		context.Background(),
		metav1.ListOptions{
			TimeoutSeconds: &timeOut,
		},
	)
	if err != nil {
		log.Print("[pods] couldn't get")
		log.Fatal(err.Error())
	}
	return rows, nil
}

func ListNodes(client kubernetes.Interface) (*core.NodeList, error) {
	rows, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		TimeoutSeconds: &timeOut,
	})
	if err != nil {
		log.Print("[nodes] couldn't get")
		log.Fatal(err.Error())
	}
	return rows, nil
}

func ListNamespaces(client kubernetes.Interface) (*core.NamespaceList, error) {
	rows, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
		LabelSelector:  namespaces_enabled_label,
		TimeoutSeconds: &timeOut,
	})
	if err != nil {
		log.Print("[namespace] couldn't get")
		log.Fatal(err.Error())
	}
	return rows, nil
}

func enabledLable(labels map[string]string) float64 {
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

func thresholdLable(labels map[string]string, threshold string, i float64) float64 {
	for key, value := range labels {
		if key == (label_theshold_prefix + threshold) {
			tmp, err := strconv.ParseFloat(value, 64)
			if err != nil {
				//todo
				// нужно задавать метрику с текущим временем
				// также нужен алерт если эта метрика есть и меняет значение за 10m
				log.Printf("[thresholdLable] failed ParseFloat: %v\n", err)
			} else {
				i = tmp
			}
		}
	}
	return i
}

func recordMetrics() {
	go func() {
		for {
			//node
			nodes, _ := ListNodes(kubeClient)
			for _, node := range nodes.Items {
				enabled := enabledLable(node.Labels)
				node_enabled.DeleteLabelValues(node.Name)
				node_enabled.WithLabelValues(node.Name).Add(enabled)
				if enabled == 1 {
					for key, value := range node_threshold_map {
						node_threshold.DeleteLabelValues(node.Name, key)
						node_threshold.WithLabelValues(node.Name, key).Add(thresholdLable(node.Labels, key, value))
					}
				}
			}

			//namespace
			namespasces, _ := ListNamespaces(kubeClient)
			for _, namespasce := range namespasces.Items {
				enabled_namespace := enabledLable(namespasce.Labels)
				namespaces_enabled.DeleteLabelValues(namespasce.Name)
				namespaces_enabled.WithLabelValues(namespasce.Name).Add(enabled_namespace)

				if enabled_namespace == 1 {
					//pod
					pods, _ := ListPods(kubeClient, namespasce.Name)
					for _, pod := range pods.Items {
						enabled_pod := enabledLable(pod.Labels)
						pod_enabled.DeleteLabelValues(namespasce.Name, pod.Name)
						pod_enabled.WithLabelValues(namespasce.Name, pod.Name).Add(enabled_pod)
						if enabled_pod == 1 {
							for key, value := range pod_threshold_map {
								pod_threshold.DeleteLabelValues(namespasce.Name, pod.Name, key)
								pod_threshold.WithLabelValues(namespasce.Name, pod.Name, key).Add(thresholdLable(pod.Labels, key, value))
							}
						}
					}
					//ingress
					ingresss, _ := ListIngress(kubeClient, namespasce.Name)
					for _, ingress := range ingresss.Items {
						enabled_ingress := enabledLable(ingress.Labels)
						ingress_enabled.DeleteLabelValues(namespasce.Name, ingress.Name)
						ingress_enabled.WithLabelValues(namespasce.Name, ingress.Name).Add(enabled_ingress)
						if enabled_ingress == 1 {
							for key, value := range pod_threshold_map {
								pod_threshold.DeleteLabelValues(namespasce.Name, ingress.Name, key)
								pod_threshold.WithLabelValues(namespasce.Name, ingress.Name, key).Add(thresholdLable(ingress.Labels, key, value))
							}
						}
					}

				}
			}

			time.Sleep(1 * 60 * time.Second)
			log.Print("Loop")
		}
	}()
}

const (
	label_theshold_prefix    = "threshold.extended-monitoring.deckhouse.io/"
	namespaces_enabled_label = "extended-monitoring.deckhouse.io/enabled"
)

var (
	timeOut      = int64(60)
	kubeClient   *kubernetes.Clientset
	reg          = prometheus.NewRegistry()
	node_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_enabled"},
		[]string{"node"},
	)
	node_threshold = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_threshold"},
		[]string{"node", "threshold"},
	)
	node_threshold_map = map[string]float64{
		"disk-bytes-warning":             70,
		"disk-bytes-critical":            80,
		"disk-inodes-warning":            90,
		"disk-inodes-critical":           95,
		"load-average-per-core-warning":  3,
		"load-average-per-core-critical": 10,
	}
	namespaces_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_enabled"},
		[]string{"namespace"},
	)
	pod_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_enabled"},
		[]string{"namespace", "pod"},
	)
	pod_threshold = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_threshold"},
		[]string{"namespace", "pod", "threshold"},
	)
	pod_threshold_map = map[string]float64{
		"disk-bytes-warning":            85,
		"disk-bytes-critical":           95,
		"disk-inodes-warning":           85,
		"disk-inodes-critical":          90,
		"container-throttling-warning":  25,
		"container-throttling-critical": 50,
	}
	ingress_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_enabled"},
		[]string{"namespace", "ingress"},
	)
	ingress_threshold = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_threshold"},
		[]string{"namespace", "ingress", "threshold"},
	)
	deployment_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_enabled"},
		[]string{"namespace", "deployment"},
	)
	deployment_threshold = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_threshold"},
		[]string{"namespace", "deployment", "threshold"},
	)
	daemonset_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_enabled"},
		[]string{"namespace", "daemonset"},
	)
	daemonset_threshold = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_threshold"},
		[]string{"namespace", "daemonset", "threshold"},
	)
	statefulset_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_enabled"},
		[]string{"namespace", "statefulset"},
	)
	statefulset_threshold = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_threshold"},
		[]string{"namespace", "statefulset", "threshold"},
	)
	cronjob_enabled = promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_cronjob_enabled"},
		[]string{"namespace", "cronjob"},
	)
)

func init() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error kubernetes config: %v\n", err)
	}
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error getting kubernetes config: %v\n", err)
	}
	prometheus.MustRegister(node_enabled)
	prometheus.MustRegister(node_threshold)
	prometheus.MustRegister(namespaces_enabled)
	prometheus.MustRegister(pod_enabled)
	prometheus.MustRegister(pod_threshold)
	prometheus.MustRegister(ingress_enabled)
	prometheus.MustRegister(ingress_threshold)
	prometheus.MustRegister(deployment_enabled)
	prometheus.MustRegister(deployment_threshold)
	prometheus.MustRegister(daemonset_enabled)
	prometheus.MustRegister(daemonset_threshold)
	prometheus.MustRegister(statefulset_enabled)
	prometheus.MustRegister(statefulset_threshold)
	prometheus.MustRegister(cronjob_enabled)
}

func main() {
	handler := promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		})
	recordMetrics()
	http.Handle("/metrics", handler)
	log.Fatal(http.ListenAndServe("127.0.0.1:8081", nil))
}
