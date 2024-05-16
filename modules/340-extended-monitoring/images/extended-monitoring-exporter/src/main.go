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
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func recordMetrics() {
	go func() {
		for {
			namespaces, err := kubeClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
				LabelSelector:  namespaces_enabled_label,
				TimeoutSeconds: &timeOut,
			})
			if err != nil {
				log.Print("[namespace] couldn't get")
				log.Fatal(err.Error())
			}
			for _, item := range namespaces.Items {
				log.Print(fmt.Sprintf("extended_monitoring_enabled{namespace=%q} 1", item.Name))
				namespaces_enabled.WithLabelValues(item.Name).Add(1)
			}

			reg.MustRegister(namespaces_enabled)
			time.Sleep(10 * 60 * time.Second)
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
	node_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_enabled"},
		[]string{"node"},
	)
	node_threshold = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_node_threshold"},
		[]string{"node", "threshold"},
	)
	namespaces_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_enabled"},
		[]string{"namespace"},
	)
	pod_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_enabled"},
		[]string{"namespace", "pod"},
	)
	pod_threshold = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_pod_threshold"},
		[]string{"namespace", "pod", "threshold"},
	)
	ingress_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_enabled"},
		[]string{"namespace", "ingress"},
	)
	ingress_threshold = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_ingress_threshold"},
		[]string{"namespace", "ingress", "threshold"},
	)
	deployment_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_enabled"},
		[]string{"namespace", "deployment"},
	)
	deployment_threshold = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_deployment_threshold"},
		[]string{"namespace", "deployment", "threshold"},
	)
	daemonset_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_enabled"},
		[]string{"namespace", "daemonset"},
	)
	daemonset_threshold = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_daemonset_threshold"},
		[]string{"namespace", "daemonset", "threshold"},
	)
	statefulset_enabled = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_enabled"},
		[]string{"namespace", "statefulset"},
	)
	statefulset_threshold = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "extended_monitoring_statefulset_threshold"},
		[]string{"namespace", "statefulset", "threshold"},
	)
)

func init() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Error kubernetes config: %v\n", err)
		os.Exit(1)
	}
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Error getting kubernetes config: %v\n", err)
		os.Exit(1)
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
