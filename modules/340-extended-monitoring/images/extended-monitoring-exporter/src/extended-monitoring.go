package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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
	Deploymentstatus                                      = "deploymentstatus"
	Deploymentreplicas                                    = "deploymentreplicas"
	Statefulsetcreated                                    = "statefulsetcreated"
	Statefulsetstatus                                     = "statefulsetstatus"
	Statefulsetreplicas                                   = "statefulsetreplicas"
	Statefulset                                           = "statefulset"
	Daemonset                                             = "daemonset"
	Podcpuusage                                           = "podcpuusage"
	Podmemory                                             = "podmemory"
	Podingress                                            = "podingress"
	Podegress                                             = "podegress"
	Daemonsetcreated                                      = "daemonsetcreated"
	Daemonsetstatus                                       = "daemonsetstatus"
	Deamonsetready                                        = "daemonsetready"
	Daemonsetavailable                                    = "daemonsetavailable"
	Daemonsetunavailable                                  = "daemonsetunavailable"
	Daemonsetunmisscheduled                               = "daemonsetunmisscheduled"
	Nodecpuusage                                          = "nodecpuusage"
	Nodememory                                            = "nodememory"
	Nodestatus                                            = "nodestatus"
	Nodeallocatable                                       = "nodeallocatable"
	Nodecondition                                         = "nodecondition"
	Nodeunschedulable                                     = "nodeunschedulable"
	NodeRole                                              = "node_role"
	NodeLabels                                            = "node_labels"
	Kube_node                                             = "node"
	PrometheusURL                                         = "http://localhost:9090"
)

var (
	podMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_pod_metric",
			Help: "Pod CPU usage",
		},
		[]string{"pod_name"},
	)

	deployment_metric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_deployment_metric",
			Help: "Deployment status",
		},
		[]string{"deployment_name"},
	)

	deployment_threshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_deployment_threshold",
			Help: "deployment_threshold",
		},
		[]string{"deployment_name"},
	)

	statefulsetMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_statefulset",
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

	podThreshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_pod_threshold",
			Help: "pod threshold",
		},
		[]string{"pod_name"},
	)

	nodeThreshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_node_threshold",
			Help: "node_threshold",
		},
		[]string{"statefulset_name"},
	)

	statefulsetThreshold = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_statefulset_threshold",
			Help: "statefulset_threshold",
		},
		[]string{"statefulset_name"},
	)

	extended_monitoring_statefulset_enabled = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "extended_monitoring_statefulset_enabled",
			Help: "statefulset_enabled",
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
	prometheus.MustRegister(deployment_metric)
	prometheus.MustRegister(statefulsetMetric)
	prometheus.MustRegister(daemonsetMetric)
	prometheus.MustRegister(podThreshold)
	prometheus.MustRegister(deployment_threshold)
	prometheus.MustRegister(nodeThreshold)
	prometheus.MustRegister(statefulsetThreshold)
	prometheus.MustRegister(extended_monitoring_statefulset_enabled)
	prometheus.MustRegister(extended_monitoring_node_enabled)
}

type exporter struct {
	client   *kubernetes.Clientset
	metrices *metrics.Clientset
}

type Exporter interface {
	ListAnnotatedObjects(namespace string) ([]Annotated, error)
	GetMetrics(objects []Annotated) error
	GetNamespace() []string
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

}

/*
This function would connect to the Kubernetes cluster and fetch objects that have been annotated, based on the namespace
and type provided.
*/

func (e *exporter) GetNamespace() []string {
	namespaces, err := e.client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("[logs] Error fetching namespaces:", err)
	}

	var ns []string
	for _, namespace := range namespaces.Items {
		ns = append(ns, namespace.Name)
	}
	return ns
}

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

	nodes, err := e.client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=true", ExtendedMonitoringEnabledLabel)})
	if err != nil {
		fmt.Println("[logs Node ] Error: ", err)
		return nil, err
	}

	// append all the objects to the annotatedObjects slice
	for _, pod := range pods.Items {

		cputhreshold := pod.Spec.DeepCopy().Overhead.Cpu()
		memorythreshold := pod.Spec.DeepCopy().Overhead.Memory()

		thresholds := make(map[string]string)
		thresholds["CPU"] = cputhreshold.String()
		thresholds["Memory"] = memorythreshold.String()

		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			Object:     "Pod",
			Thresholds: thresholds,
		})

		// go func(namespace, podName string) {
		podMetrics(namespace, pod.Name)
		// }(pod.Namespace, pod.Name)
	}

	for _, deployment := range deployments.Items {

		cputhreshold := deployment.Spec.DeepCopy().Template.Spec.Overhead.Cpu()
		memorythreshold := deployment.Spec.DeepCopy().Template.Spec.Overhead.Memory()
		replicas := deployment.Spec.DeepCopy().Replicas

		thresolds := make(map[string]string)

		thresolds["CPU"] = cputhreshold.String()
		thresolds["Memory"] = memorythreshold.String()
		thresolds["Replicas"] = string(*replicas)

		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace:  deployment.Namespace,
			Name:       deployment.Name,
			Object:     "Deployment",
			Thresholds: thresolds,
		})

		// go func(namespace, deploymentName string) {
		deploymentMetrics(namespace, deployment.Name)
		// }(deployment.Namespace, deployment.Name)
	}

	// for _, ingress := range ingresses.Items {
	// 	annotatedObjects = append(annotatedObjects, Annotated{
	// 		Namespace: ingress.Namespace,
	// 		Name:      ingress.Name,
	// 		Object:    "Ingress",
	// 	})
	// }

	for _, stateful := range stateful.Items {
		cputhreshold := stateful.Spec.DeepCopy().Template.Spec.Overhead.Cpu()
		memorythreshold := stateful.Spec.DeepCopy().Template.Spec.Overhead.Memory()
		replicas := stateful.Spec.DeepCopy().Replicas

		thresolds := make(map[string]string)

		thresolds["CPU"] = cputhreshold.String()
		thresolds["Memory"] = memorythreshold.String()
		thresolds["Replicas"] = string(*replicas)

		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace:  stateful.Namespace,
			Name:       stateful.Name,
			Object:     "StatefulSet",
			Thresholds: thresolds,
		})

		// go func(namespace, statefulsetName string) {
		statefulsetMetrics(namespace, stateful.Name)
		// }(stateful.Namespace, stateful.Name)
	}

	for _, daemonset := range daemons.Items {

		cputhreshold := daemonset.Spec.DeepCopy().Template.Spec.Overhead.Cpu()
		memorythreshold := daemonset.Spec.DeepCopy().Template.Spec.Overhead.Memory()

		thresolds := make(map[string]string)

		thresolds["CPU"] = cputhreshold.String()
		thresolds["Memory"] = memorythreshold.String()

		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace: daemonset.Namespace,
			Name:      daemonset.Name,
			Object:    "DaemonSet",
		})

		// go func(namespace, daemonsetName string) {
		daemonsetMetrics(namespace, daemonset.Name)
		// }(daemonset.Namespace, daemonset.Name)
	}

	for _, node := range nodes.Items {
		annotatedObjects = append(annotatedObjects, Annotated{
			Namespace: node.Namespace,
			Name:      node.Name,
			Object:    "Node",
		})

		fmt.Println(node.Name)

		// go func(nodeName string) {
		nodeMetrics(node.Name)
		// }(node.Name)
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
var PODS []Annotated
var DEPLOYMENTS []Annotated
var STATEFULSETS []Annotated
var DAEMONSETS []Annotated
var NODES []Annotated

func main() {
	exporter := NewExporter()
	done := make(chan bool)

	go func() {
		for {
			ns := exporter.GetNamespace()
			for _, namespace := range ns {
				objects, err := exporter.ListAnnotatedObjects(namespace)
				if err != nil {
					fmt.Println(time.Now(), "[logs] Error fetching annotated objects:", err)
				}
				if len(objects) > 0 {
					SLICE = objects
				}
			}
		}
	}()
	var wg sync.WaitGroup

	// extract the objects from slice to the individual slices
	// go func() {
	// 	for {
	// 		for _, object := range SLICE {
	// 			if object.Object == Pod {
	// 				PODS = append(PODS, object)
	// 			}
	// 			if object.Object == Deployment {
	// 				DEPLOYMENTS = append(DEPLOYMENTS, object)
	// 			}
	// 			if object.Object == Statefulset {
	// 				STATEFULSETS = append(STATEFULSETS, object)
	// 			}
	// 			if object.Object == Daemonset {
	// 				DAEMONSETS = append(DAEMONSETS, object)
	// 			}
	// 		}
	// 	}
	// }()

	fmt.Println("Starting server on port http://localhost:8080")

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)

	wg.Wait()

	done <- true
}

func podMetrics(namespace, podName string) {

	qeuryCPU := fmt.Sprintf(
		`container_cpu_usage_seconds_total{pod="%s",namespace="%s"}`,
		podName,
		namespace,
	)

	qeuryMemory := fmt.Sprintf(
		`container_memory_usage_bytes{pod="%s",namespace="%s"}`,
		podName,
		namespace,
	)
	queryEgress := fmt.Sprintf(
		`container_network_transmit_bytes_total{pod="%s",interface="eth0"}`,
		podName,
	)

	queryIngress := fmt.Sprintf(
		`container_network_receive_bytes_total{pod="%s",interface="eth0"}`,
		podName,
	)

	go query(qeuryCPU, Podcpuusage, podName, namespace)
	go query(qeuryMemory, Podmemory, podName, namespace)
	go query(queryEgress, Podegress, podName, namespace)
	go query(queryIngress, Podingress, podName, namespace)
}

func deploymentMetrics(namespace, deploymentName string) {
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

	go query(qeuryStatus, Deploymentstatus, deploymentName, namespace)
	go query(qeuryReplicas, Deploymentreplicas, deploymentName, namespace)

}

func statefulsetMetrics(namespace, statefulsetName string) {

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

	go query(qeuryCreated, Statefulsetcreated, statefulsetName, namespace)
	go query(qeuryStatus, Statefulsetstatus, statefulsetName, namespace)
	go query(qeuryReplicas, Statefulsetreplicas, statefulsetName, namespace)

}

func daemonsetMetrics(namespace, daemonsetName string) {
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

	go query(qeuryCreated, Daemonsetcreated, daemonsetName, namespace)
	go query(qeuryStatus, Daemonsetstatus, daemonsetName, namespace)
	go query(qeuryAvailable, Daemonsetavailable, daemonsetName, namespace)
	go query(qeuryUnmisscheduled, Daemonsetunmisscheduled, daemonsetName, namespace)
	go query(qeuryMisscheduled, Daemonsetunmisscheduled, daemonsetName, namespace)

}

func nodeMetrics(nodeName string) {
	query_nodestatus := fmt.Sprintf(
		`kube_node_status_capacity{unit="core", node="%s"}`,
		nodeName,
	)
	query_node_status_allocatable := fmt.Sprintf(
		`kube_node_status_allocatable{unit="core", node="%s"}`,
		nodeName,
	)

	query_node_status_condition := fmt.Sprintf(
		`kube_node_status_condition{node="%s"}`,
		nodeName,
	)

	query_node_spec_unschedulable := fmt.Sprintf(
		`kube_node_spec_unschedulable{node="%s"}`,
		nodeName,
	)

	query_node_role := fmt.Sprintf(
		`kube_node_role{node="%s"}`,
		nodeName,
	)

	query_node_labels := fmt.Sprintf(
		`kube_node_labels{node="%s"}`,
		nodeName,
	)

	go query(query_nodestatus, Nodestatus, nodeName, "")
	go query(query_node_status_allocatable, Nodeallocatable, nodeName, "")
	go query(query_node_status_condition, Nodecondition, nodeName, "")
	go query(query_node_spec_unschedulable, Nodeunschedulable, nodeName, "")
	go query(query_node_role, NodeRole, nodeName, "")
	go query(query_node_labels, NodeLabels, nodeName, "")
}

func query(query string, objectType string, objName string, namespace string) {
	client, err := api.NewClient(api.Config{
		Address: PrometheusURL,
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
			case Podcpuusage:
				podMetric.WithLabelValues(objName + "-cpuusage").Set(latestValue)
			case Podmemory:
				podMetric.WithLabelValues(objName + "-memory").Set(latestValue)
			case Podingress:
				podMetric.WithLabelValues(objName + "-ingress").Set(latestValue)
			case Podegress:
				podMetric.WithLabelValues(objName + "-egress").Set(latestValue)
			case Deploymentstatus:
				deployment_metric.WithLabelValues(objName + "-status").Set(latestValue)
			case Deploymentreplicas:
				deployment_threshold.WithLabelValues(objName + "-replicas").Set(latestValue)
			case Statefulsetcreated:
				statefulsetMetric.WithLabelValues(objName + "-created").Set(latestValue)
			case Statefulsetstatus:
				statefulsetMetric.WithLabelValues(objName + "-status").Set(latestValue)
			case Statefulsetreplicas:
				statefulsetMetric.WithLabelValues(objName + "-replicas").Set(latestValue)
			case Daemonsetcreated:
				daemonsetMetric.WithLabelValues(objName + "-created").Set(latestValue)
			case Daemonsetstatus:
				daemonsetMetric.WithLabelValues(objName + "-status").Set(latestValue)
			case Daemonsetavailable:
				daemonsetMetric.WithLabelValues(objName + "-available").Set(latestValue)
			case Daemonsetunmisscheduled:
				daemonsetMetric.WithLabelValues(objName + "-unmisscheduled").Set(latestValue)
			case Daemonsetunavailable:
				daemonsetMetric.WithLabelValues(objName + "-unavailable").Set(latestValue)
			case Nodestatus:
				nodeThreshold.WithLabelValues(objName + "-status").Set(latestValue)
			case Nodeallocatable:
				nodeThreshold.WithLabelValues(objName + "-allocatable").Set(latestValue)
			case Nodecondition:
				nodeThreshold.WithLabelValues(objName + "-condition").Set(latestValue)
			case Nodeunschedulable:
				nodeThreshold.WithLabelValues(objName + "-unschedulable").Set(latestValue)
			case NodeRole:
				podMetric.WithLabelValues(objName + "-role").Set(latestValue)
			case NodeLabels:
				nodeThreshold.WithLabelValues(objName + "-labels").Set(latestValue)
			default:
				fmt.Println("invalid object type: ", objectType)
			}
		}
	}()
}
