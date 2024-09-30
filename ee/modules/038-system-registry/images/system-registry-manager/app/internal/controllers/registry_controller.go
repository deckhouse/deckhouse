package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

type EmbeddedRegistry struct {
	UserAccounts        []UserAccount
	RegistryPolices     []RegistryPolicy
	RegistryPods        []corev1.Pod
	MasterNodes         []corev1.Node
	RegistryManagerPods []corev1.Pod
}

type UserAccount struct {
}
type RegistryPolicy struct {
}

type MasterNode struct {
}

const RegistryManagerPodsLabel string = "app=system-registry-manager"
const EmbeddedRegistryPodsLabel string = "component=system-registry"
const MasterNodesLabel string = "node-role.kubernetes.io/master="

type RegistryReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	KubeClient       *kubernetes.Clientset
	Recorder         record.EventRecorder
	EmbeddedRegistry EmbeddedRegistry
}

func (r *RegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log.Log.Info("Reconciling Registry", "name", req.Name)
	return ctrl.Result{}, nil

	err := r.checkAndDeployComponents()
	if err != nil {
		log.Log.Error(err, "Failed to check and deploy components")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r *RegistryReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	masterNodesFactory := informers.NewSharedInformerFactoryWithOptions(r.KubeClient, 0*time.Second,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = "node-role.kubernetes.io/master="
		}),
	)
	nodesInformer := masterNodesFactory.Core().V1().Nodes().Informer()

	// Add event handler for nodes
	_, err := nodesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    r.handleMasterNodeAdd,
		UpdateFunc: r.handleMasterNodeUpdate,
		DeleteFunc: r.handleMasterNodeDelete,
	})
	if err != nil {
		return err
	}

	managerPodPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		pod, ok := object.(*corev1.Pod)
		if !ok {
			return false
		}
		//log.Log.Info("DEBUG", "name", pod.Name, "labels", pod.Labels)
		value, exists := pod.Labels["app"]
		return exists && value == "system-registry-manager"
	})

	go nodesInformer.Run(ctx.Done())
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(managerPodPredicate).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

func (r *RegistryReconciler) checkAndDeployComponents() error {

	// Fill registry manager pods
	registryManagerPods, err := r.KubeClient.CoreV1().Pods("d8-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: RegistryManagerPodsLabel,
	})
	if err != nil {
		ctrl.Log.Error(err, "Error listing registry manager pods")
		return err
	}

	for _, pod := range registryManagerPods.Items {
		r.EmbeddedRegistry.RegistryManagerPods = append(r.EmbeddedRegistry.RegistryManagerPods, pod)
	}

	// Fill Registry Pods
	registryPods, err := r.KubeClient.CoreV1().Pods("d8-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: EmbeddedRegistryPodsLabel,
	})
	if err != nil {
		ctrl.Log.Error(err, "Error listing embedded registry pods")
		return err
	}

	for _, pod := range registryPods.Items {
		r.EmbeddedRegistry.RegistryPods = append(r.EmbeddedRegistry.RegistryPods, pod)
	}

	// Fill Master Nodes
	masterNodes, err := r.KubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: MasterNodesLabel,
	})
	if err != nil {
		ctrl.Log.Error(err, "Error listing master nodes")
		return err
	}

	for _, node := range masterNodes.Items {
		r.EmbeddedRegistry.MasterNodes = append(r.EmbeddedRegistry.MasterNodes, node)
	}

	// TESTS
	for _, node := range r.EmbeddedRegistry.MasterNodes {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}

		ctrl.Log.Info("masterNodes:", "name", node.Name, "NodeReady", isReady)
	}

	for _, pod := range r.EmbeddedRegistry.RegistryPods {
		ctrl.Log.Info("EmbeddedRegistryPods:", "name", pod.Name, "Phase", pod.Status.Phase,
			"node", pod.Spec.NodeName)
	}

	for _, pod := range r.EmbeddedRegistry.RegistryManagerPods {
		ctrl.Log.Info("RegistryManagerPod:", "name", pod.Name, "Phase", pod.Status.Phase,
			"node", pod.Spec.NodeName)
		podIP := pod.Status.PodIP
		if podIP != "" {
			//	r.Recorder.Event(&pod, corev1.EventTypeNormal, "PodReconciled", fmt.Sprintf("Pod %s reconciled successfully", pod.Name))

			apiURL := fmt.Sprintf("https://%s:4577", podIP)
			if err := r.createStaticPod(apiURL, pod.Spec.NodeName); err != nil {
				ctrl.Log.Error(err, "Failed to create static pod")
			}
		}
	}

	manageComponents()
	return nil

}

func (r *RegistryReconciler) createStaticPod(apiURL string, nodeName string) error {
	resp, err := http.Post(fmt.Sprintf("%s/staticpod/create", apiURL), "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to create static pod: %w", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	ctrl.Log.Info("Create static pod response:", "Node", nodeName, "response", string(body))
	return nil
}

func manageComponents() {
	// Пример логики управления Docker Registry
	manageDockerRegistry()

	// Логика управления Docker Distribution
	manageDockerDistribution()

	// Логика управления Docker Auth
	manageDockerAuth()
}

func manageDockerRegistry() {
	resp, err := http.Get("http://192.168.1.201:5001/")
	if err != nil {
		ctrl.Log.Error(err, "Error getting Docker Registry status")
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctrl.Log.Error(err, "Error reading response")
		return
	}
	ctrl.Log.Info("Docker Registry status:", "status", string(body))
}

func manageDockerDistribution() {
	// Пример логики управления Docker Distribution
	ctrl.Log.Info("Managing Docker Distribution")
}

func manageDockerAuth() {
	// Пример логики управления Docker Auth
	ctrl.Log.Info("Managing Docker Auth")
}

func (r *RegistryReconciler) handleMasterNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	ctrl.Log.Info("Node added", "name", node.Name, "namespace", node.Namespace)
}

func (r *RegistryReconciler) handleMasterNodeUpdate(oldObj, newObj interface{}) {
	oldNode := oldObj.(*corev1.Node)
	newNode := newObj.(*corev1.Node)

	oldReady := getNodeConditionStatus(oldNode, corev1.NodeReady)
	newReady := getNodeConditionStatus(newNode, corev1.NodeReady)

	if oldReady != newReady {
		log.Log.Info("Node readiness changed", "name", newNode.Name, "oldReady", oldReady, "newReady", newReady)
	}
}
func (r *RegistryReconciler) handleMasterNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	log.Log.Info("Node deleted", "name", node.Name, "namespace", node.Namespace)
	// Ваш код для обработки удаления узла
}

func getNodeConditionStatus(node *corev1.Node, conditionType corev1.NodeConditionType) corev1.ConditionStatus {
	for _, condition := range node.Status.Conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}
