package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/prometheus/client_golang/prometheus"
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
	deploymentstatus                                      = "deploymentstatus"
	deploymentreplicas                                    = "deploymentreplicas"
	statefulsetcreated                                    = "statefulsetcreated"
	statefulsetstatus                                     = "statefulsetstatus"
	statefulsetreplicas                                   = "statefulsetreplicas"
	podcpuusage                                           = "pod-cpuusage"
	podmemory                                             = "pod-memory"
	daemonsetcreated                                      = "daemonsetcreated"
	daemonsetstatus                                       = "daemonsetstatus"
	deamonsetready                                        = "daemonsetready"
	daemonsetavailable                                    = "daemonsetavailable"
	daemonsetunavailable                                  = "daemonsetunavailable"
	daemonsetunmisscheduled                               = "daemonsetunmisscheduled"
	nodecpuusage                                          = "nodecpuusage"
	nodememory                                            = "nodememory"
	kube_node                                             = "node"
)

type exporter struct {
	client   *kubernetes.Clientset
	metrices *metrics.Clientset
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

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error building metrics client: %v\n", err)
		os.Exit(1)
	}

	return &exporter{
		client:   clientset,
		metrices: metricsClient,
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

// to expose the metrices we use prometheus client library
// we can run queries on the metrices using promql

var (
	podMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pod_metrices",
			Help: "Pod CPU usage",
		},
		[]string{"pod_name"},
	)

	deploymentMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_deployment_threshold",
			Help: "Deployment status",
		},
		[]string{"deployment_name"},
	)
	statefulsetMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_statefulset_threshold",
			Help: "Statefulset status",
		},
		[]string{"statefulset_name"},
	)
	daemonsetMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_daemonset_threshold",
			Help: "Daemonset status",
		},
		[]string{"daemonset_name"},
	)

	extended_monitoring_pod_threshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_pod_threshold",
			Help: "pod threshold",
		},
		[]string{"pod_name"},
	)

	deployment_threshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_deployment_threshold",
			Help: "deployment_threshold",
		},
		[]string{"deployment_name"},
	)

	extended_monitoring_node_threshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_node_threshold",
			Help: "node_threshold",
		},
		[]string{"statefulset_name"},
	)

	extended_monitoring_statefulset_threshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_statefulset_threshold",
			Help: "statefulset_threshold",
		},
		[]string{"statefulset_name"},
	)

	extended_monitoring_statefulset_enabled = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_statefulset_threshold",
			Help: "statefulset_threshold",
		},
		[]string{"statefulset_name"},
	)

	extended_monitoring_node_enabled = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_node_enabled",
			Help: "is_node_enabled?",
		},
		[]string{"node_name"},
	)
)

func init() {
	prometheus.MustRegister(podMetric)
	prometheus.MustRegister(deploymentMetric)
	prometheus.MustRegister(statefulsetMetric)
	prometheus.MustRegister(daemonsetMetric)
	prometheus.MustRegister(extended_monitoring_pod_threshold)
	prometheus.MustRegister(deployment_threshold)
	prometheus.MustRegister(extended_monitoring_node_threshold)
	prometheus.MustRegister(extended_monitoring_statefulset_threshold)
	prometheus.MustRegister(extended_monitoring_statefulset_enabled)
	prometheus.MustRegister(extended_monitoring_node_enabled)
}

func cpuUsage(prometheusURL, namespace, podName string) {
	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		panic(err)
	}

	apiClient := v1.NewAPI(client)

	query := fmt.Sprintf(
		`container_cpu_usage_seconds_total{pod="%s"}`,
		podName,
	)

	go func() {
		for {
			result, warning, err := apiClient.QueryRange(context.Background(), query, v1.Range{
				Start: time.Now().Add(-2 * time.Second),
				End:   time.Now(),
				Step:  (2 * time.Second) / 2,
			})

			if err != nil {
				fmt.Println("Error querying Prometheus:", err)
				continue
			}

			if warning != nil {
				fmt.Println("Warning querying Prometheus:", warning)
			}

			var latestValue float64
			if result != nil && result.Type() == model.ValMatrix {
				matrix := result.(model.Matrix)
				if len(matrix) > 0 {
					latestValue = float64(matrix[matrix.Len()-1].Values[0].Value)
					fmt.Printf("Pod: %s, Time: %s, Value: %f\n", podName, matrix[len(matrix)-1].Values[0].Timestamp.Time().Format(time.RFC3339), latestValue)
				}
			}

			podMetric.WithLabelValues(podName + "-cpuusage").Set(latestValue)
		}
	}()
}

func memUsage(prometheusURL, namespace, podName string) {
	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		panic(err)
	}

	apiClient := v1.NewAPI(client)

	query := fmt.Sprintf(
		`container_memory_usage_bytes{pod="%s"}`,
		podName,
	)

	go func() {
		for {
			result, warning, err := apiClient.QueryRange(context.Background(), query, v1.Range{
				Start: time.Now().Add(-2 * time.Second),
				End:   time.Now(),
				Step:  (2 * time.Second) / 2,
			})

			if err != nil {
				fmt.Println("Error querying Prometheus:", err)
				continue
			}

			if warning != nil {
				fmt.Println("Warning querying Prometheus:", warning)
			}

			var latestValue float64
			if result != nil && result.Type() == model.ValMatrix {
				matrix := result.(model.Matrix)
				if len(matrix) > 0 {
					// the default memory format is in bytes -- MB
					latestValue = float64(matrix[matrix.Len()-1].Values[0].Value) / 1000000
					// latestValue = float64(matrix[matrix.Len()-1].Values[0].Value)
					fmt.Printf("Pod Mem: %s, Time: %s, Value: %f\n", podName, matrix[len(matrix)-1].Values[0].Timestamp.Time().Format(time.RFC3339), latestValue)
				}
			}

			podMetric.WithLabelValues(podName + "-memory").Set(latestValue)
		}
	}()
}

func deploymentMetrics(prometheusURL, namespace, deploymentName string) {
	qeuryStatus := fmt.Sprintf(
		`kube_deployment_status_replicas_available{namespace="%s", deployment="%s"}`,
		namespace,
		deploymentName,
	)

	qeuryReplicas := fmt.Sprintf(
		`kube_deployment_status_replicas{deployment="%s",namespace="%s"}`,
		deploymentName,
		namespace,
	)

	go query(prometheusURL, qeuryStatus, deploymentstatus, deploymentName, namespace)
	go query(prometheusURL, qeuryReplicas, deploymentreplicas, deploymentName, namespace)

}

func statefulsetMetrics(prometheusURL, namespace, statefulsetName string) {

	qeuryCreated := fmt.Sprintf(
		`kube_statefulset_created{namespace="%s", statefulset="%s"}`,
		namespace,
		statefulsetName,
	)

	qeuryStatus := fmt.Sprintf(
		`kube_statefulset_status_replicas_available{namespace="%s", statefulset="%s"}`,
		namespace,
		statefulsetName,
	)

	qeuryReplicas := fmt.Sprintf(
		`kube_statefulset_status_replicas{namespace="%s", statefulset="%s"}`,
		namespace,
		statefulsetName,
	)

	go query(prometheusURL, qeuryCreated, statefulsetcreated, statefulsetName, namespace)
	go query(prometheusURL, qeuryStatus, statefulsetstatus, statefulsetName, namespace)
	go query(prometheusURL, qeuryReplicas, statefulsetreplicas, statefulsetName, namespace)

}

func daemonsetMetrics(prometheusURL, namespace, daemonsetName string) {
	qeuryCreated := fmt.Sprintf(
		`kube_daemonset_created{namespace="%s", daemonset="%s"}`,
		namespace,
		daemonsetName,
	)

	qeuryStatus := fmt.Sprintf(
		`kube_daemonset_status_number_ready{namespace="%s", daemonset="%s"}`,
		namespace,
		daemonsetName,
	)

	qeuryAvailable := fmt.Sprintf(
		`kube_daemonset_status_number_available{namespace="%s", daemonset="%s"}`,
		namespace,
		daemonsetName,
	)

	qeuryUnmisscheduled := fmt.Sprintf(
		`kube_daemonset_status_number_unavailable{namespace="%s", daemonset="%s"}`,
		namespace,
		daemonsetName,
	)

	qeuryMisscheduled := fmt.Sprintf(
		`kube_daemonset_status_number_misscheduled{namespace="%s", daemonset="%s"}`,
		namespace,
		daemonsetName,
	)

	go query(prometheusURL, qeuryCreated, daemonsetcreated, daemonsetName, namespace)
	go query(prometheusURL, qeuryStatus, daemonsetstatus, daemonsetName, namespace)
	go query(prometheusURL, qeuryAvailable, daemonsetavailable, daemonsetName, namespace)
	go query(prometheusURL, qeuryUnmisscheduled, daemonsetunmisscheduled, daemonsetName, namespace)
	go query(prometheusURL, qeuryMisscheduled, daemonsetunmisscheduled, daemonsetName, namespace)

}

func nodeMetrics(prometheusURL, namespace, nodeName string) {
	queryString := fmt.Sprintf(
		`kube_node_info`,
	)
	go query(prometheusURL, queryString, kube_node, nodeName, namespace)
}

func query(prometheusURL, query string, objectType string, objName string, namespace string) {
	client, err := api.NewClient(api.Config{
		Address: prometheusURL,
	})
	if err != nil {
		panic(err)
	}

	apiClient := v1.NewAPI(client)

	go func() {
		for {
			result, warning, err := apiClient.QueryRange(context.Background(), query, v1.Range{
				Start: time.Now().Add(-2 * time.Second),
				End:   time.Now(),
				Step:  (2 * time.Second) / 2,
			})

			if err != nil {
				fmt.Println("Error querying Prometheus:", err)
				fmt.Println(objectType)
				continue
			}

			if warning != nil {
				fmt.Println("Warning querying Prometheus:", warning)
			}
			// fmt.Println(result)
			var latestValue float64
			if result != nil && result.Type() == model.ValMatrix {
				matrix := result.(model.Matrix)
				// fmt.Println(matrix)
				if len(matrix) > 0 {
					latestValue = float64(matrix[matrix.Len()-1].Values[0].Value)
					// fmt.Printf("Deployment: %s, Time: %s, Value: %f\n", objectType, matrix[len(matrix)-1].Values[0].Timestamp.Time().Format(time.RFC3339), latestValue)
				}
			}
			// fmt.Println(latestValue)
			switch objectType {
			case "podcpuusage":
				podMetric.WithLabelValues(objName + "-cpuusage").Set(latestValue)
			case "podmemory":
				podMetric.WithLabelValues(objName + "-memory").Set(latestValue)
			case "deploymentstatus":
				deploymentMetric.WithLabelValues(objName + "-status").Set(latestValue)
			case "deploymentreplicas":
				deploymentMetric.WithLabelValues(objName + "-replicas").Set(latestValue)
			case "statefulsetcreated":
				statefulsetMetric.WithLabelValues(objName + "-created").Set(latestValue)
			case "statefulsetstatus":
				statefulsetMetric.WithLabelValues(objName + "-status").Set(latestValue)
			case "statefulsetreplicas":
				statefulsetMetric.WithLabelValues(objName + "-replicas").Set(latestValue)
			case "daemonsetcreated":
				daemonsetMetric.WithLabelValues(objName + "-created").Set(latestValue)
			case "daemonsetstatus":
				daemonsetMetric.WithLabelValues(objName + "-status").Set(latestValue)
			case "daemonsetavailable":
				daemonsetMetric.WithLabelValues(objName + "-available").Set(latestValue)
			case "daemonsetunmisscheduled":
				daemonsetMetric.WithLabelValues(objName + "-unmisscheduled").Set(latestValue)
			case "daemonsetunavailable":
				daemonsetMetric.WithLabelValues(objName + "-unavailable").Set(latestValue)
			default:
				fmt.Println("invalid object type")
			}
		}
	}()
}
