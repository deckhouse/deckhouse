package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	DeprecatedExtendedMonitoringAnnotationThresholdPrefix = "threshold.extended-monitoring.flant.com/"
	DeprecatedExtendedMonitoringEnabledAnnotation         = "extended-monitoring.flant.com/enabled"
	ExtendedMonitoringLabelThresholdPrefix                = "threshold.extended-monitoring.deckhouse.io/"
	ExtendedMonitoringEnabledLabel                        = "extended-monitoring.deckhouse.io/enabled"
	DefaultServerAddress                                  = "0.0.0.0"
	DefaultPort                                           = 8080
	Pod                                                   = "pod"
	Deployment                                            = "deployment"
)

type exporter struct {
	client *kubernetes.Clientset
}
type Exporter interface {
	ListAnnotatedObjects(namespace string) ([]Annotated, error)
	GetMetrics(objects []Annotated) error
}

func NewExporter() Exporter {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error()) // Handle error properly in your application.
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create clientset: %v", err)
	}

	return &exporter{
		client: clientset,
	}
}

/*
This function would connect to the Kubernetes cluster and fetch objects that have been annotated, based on the namespace
and type provided.
*/
func (e *exporter) ListAnnotatedObjects(namespace string) ([]Annotated, error) {
	// get the list of objects that have been annotated on the namespaec given with value : extended-monitoring.deckhouse.io/enabled=true
	var annotatedObjects []Annotated
	pods, err := e.client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", ExtendedMonitoringEnabledLabel)})
	if err != nil {
		fmt.Println("[logs Pod ] Error: ", err)
		return nil, err
	}
	deployments, err := e.client.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", ExtendedMonitoringEnabledLabel)})
	if err != nil {
		fmt.Println("[logs Deployment ] Error: ", err)
		return nil, err
	}
	// ingresses, err := e.client.ExtensionsV1beta1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", ExtendedMonitoringEnabledLabel)})
	// if err != nil {
	// 	fmt.Println("[logs Ingress ] Error: ", err)
	// 	return nil, err
	// }
	stateful, err := e.client.AppsV1().StatefulSets(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", ExtendedMonitoringEnabledLabel)})
	if err != nil {
		fmt.Println("[logs Stateful ] Error: ", err)
		return nil, err
	}
	daemons, err := e.client.AppsV1().DaemonSets(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", ExtendedMonitoringEnabledLabel)})
	if err != nil {
		fmt.Println("[logs Daemon ] Error: ", err)
		return nil, err
	}

	// append all the objects to the annotatedObjects slice
	for _, pod := range pods.Items {
		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Object:    "Pod",
		})
	}

	for _, deployment := range deployments.Items {
		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace: deployment.Namespace,
			Name:      deployment.Name,
			Object:    "Deployment",
		})
	}

	// for _, ingress := range ingresses.Items {
	// 	annotatedObjects = append(annotatedObjects, Annotated{
	// 		Namespace: ingress.Namespace,
	// 		Name:      ingress.Name,
	// 		Object:    "Ingress",
	// 	})
	// }

	for _, stateful := range stateful.Items {
		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace: stateful.Namespace,
			Name:      stateful.Name,
			Object:    "StatefulSet",
		})
	}

	for _, daemonset := range daemons.Items {
		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace: daemonset.Namespace,
			Name:      daemonset.Name,
			Object:    "DaemonSet",
		})
	}

	return annotatedObjects, nil

}

func (e *exporter) GetMetrics(objects []Annotated) error {

	return nil
}

type Annotated struct {
	Namespace  string
	Name       string
	Enabled    bool
	Object     string
	Thresholds map[string]string
}

func NewAnnotated() *Annotated {
	return &Annotated{}
}

// # Monitoring is enabled by default for all controllers in a namespace and can be disabled by using
// # the 'extended-monitoring.deckhouse.io/enabled=false' label
// # or the 'extended-monitoring.flant.com/enabled=false' annotation.
func (a *Annotated) IsEnabled(kube_labels map[string]string, annotations string) bool {
	if enabled, ok := kube_labels[ExtendedMonitoringEnabledLabel]; ok {
		return enabled == "true"
	} else if enabled, ok := kube_labels[DeprecatedExtendedMonitoringEnabledAnnotation]; ok {
		return enabled == "true"
	} else if enabled, ok := kube_labels[annotations]; ok {
		return enabled == "true"
	}

	return false

}

func (a *Annotated) ParseThresholds(labels map[string]string, annotations map[string]string, defaultThresholds map[string]string) map[string]string {
	thresholds := make(map[string]string)

	for name, value := range annotations {
		if strings.HasPrefix(name, DeprecatedExtendedMonitoringAnnotationThresholdPrefix) {
			prefixedName := strings.Replace(name, DeprecatedExtendedMonitoringAnnotationThresholdPrefix, "", 1)
			thresholds[prefixedName] = value
		}
	}

	for name, value := range labels {
		if strings.HasPrefix(name, ExtendedMonitoringLabelThresholdPrefix) {
			prefixedName := strings.Replace(name, ExtendedMonitoringLabelThresholdPrefix, "", 1)
			thresholds[prefixedName] = value
		}
	}

	return thresholds
}

var SLICE []Annotated

func main() {
	exporter := NewExporter()
	// this done is used to signal the main goroutine to exit
	done := make(chan bool)

	// go routine to fetch the annotated objects every 5 seconds
	go func() {
		for {
			objects, err := exporter.ListAnnotatedObjects("default")
			if err != nil {
				fmt.Println("[logs] Error fetching annotated objects:", err)
			} else {
				// print time also
				fmt.Println("[logs: ", time.Now(), "] Annotated objects:", objects)
			}

			SLICE = objects

			time.Sleep(5 * time.Second)
		}
	}()

	// this handler exposes the annotated objects as JSON on the web
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jsonData, err := json.MarshalIndent(SLICE, "", "  ")
		if err != nil {
			http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
			return
		}
		w.Write(jsonData)
	})

	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("[logs: ", time.Now(), "] Starting HTTP server on port 8080...")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("[logs: %s] Error starting HTTP server: %s", time.Now(), err)
	}

	// this done is used to signal the main goroutine to exit like a graceful shutdown
	// when the server is stopped it will send a true to the done channel and it will exit
	// but as we have a infinite loop in the go routine it will never exit
	// and if we dont use this the main goroutine will exit and the go routine will also exit which will stop the server
	done <- true
}

// we will use k8s metrics server to get the metrics and then we will use the prometheus client to expose the metrics
func (exporter *exporter) cpu_uses(Type string, Namespace string, Name string) (float64, error) {
	// get the metrics for the object

	metrics, err := exporter.client.CoreV1().Pods(Namespace).GetMetrics(context.Background(), Name, metav1.GetOptions{})
	if err != nil {
		fmt.Println("[logs] Error fetching metrics:", err)
		return 0, err
	}

	for _, container := range metrics.Containers {
		if container.Name == "nginx" {
			return container.Usage.Cpu().MilliValue(), nil
		}
	}

	return 0, nil

}
