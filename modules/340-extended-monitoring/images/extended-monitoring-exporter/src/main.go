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
			list, err := kubeClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
				LabelSelector:  EXTENDED_MONITORING_ENABLED_LABEL,
				TimeoutSeconds: &timeOut,
			})
			if err != nil {
				log.Fatal(err.Error())
			}
			httpReqs := prometheus.NewCounterVec(
				prometheus.CounterOpts{Name: "extended_monitoring_enabled"},
				[]string{"namespace"},
			)
			prometheus.MustRegister(httpReqs)
			for _, item := range list.Items {
				log.Print(fmt.Sprintf("extended_monitoring_enabled{namespace=%q} 1", item.Name))
				httpReqs.WithLabelValues(item.Name).Add(1)
			}

			reg.MustRegister(httpReqs)
			time.Sleep(10 * 60 * time.Second)
		}
	}()
}

var (
	//    540 extended_monitoring_pod_threshold
	//     90 extended_monitoring_pod_enabled
	//     52 extended_monitoring_ingress_threshold
	//     45 extended_monitoring_deployment_threshold
	//     45 extended_monitoring_deployment_enabled
	//     26 extended_monitoring_ingress_enabled
	//     14 extended_monitoring_daemonset_threshold
	//     14 extended_monitoring_daemonset_enabled
	//      3 extended_monitoring_statefulset_threshold
	//      3 extended_monitoring_statefulset_enabled
	//      3 extended_monitoring_node_enabled
	//     18 extended_monitoring_node_threshold

	EXTENDED_MONITORING_LABEL_THRESHOLD_PREFIX = "threshold.extended-monitoring.deckhouse.io/"
	EXTENDED_MONITORING_ENABLED_LABEL          = "extended-monitoring.deckhouse.io/enabled"
	kubeClient                                 *kubernetes.Clientset
	timeOut                                    = int64(60)
	reg                                        = prometheus.NewRegistry()
)

func init() {
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error kubernetes config: %v\n", err)
		os.Exit(1)
	}
	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error getting kubernetes config: %v\n", err)
		os.Exit(1)
	}
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
