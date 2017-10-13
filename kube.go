package main

import (
	"regexp"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/romana/rlog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const KubeTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
const KubeNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
const KubeDefaultNamespace = "antiopa"
const KubeAntiopaDeploymentName = "antiopa"
const KubeAntiopaContainerName = "antiopa"

var (
	KubernetesClient           *kubernetes.Clientset
	KubernetesAntiopaNamespace string
)

// InitKube - инициализация kubernetes клиента
// Можно подключить изнутри, а можно на основе .kube директории
func InitKube() {
	rlog.Info("KUBE Init Kubernetes client")

	var err error
	var config *rest.Config

	if _, err := os.Stat(KubeTokenFile); os.IsNotExist(err) {
		rlog.Info("KUBE-INIT Connecting to kubernetes out-of-cluster")

		var kubeconfig string
		if kubeconfig = os.Getenv("KUBECONFIG"); kubeconfig == "" {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
		rlog.Infof("KUBE-INIT Using kube config at %s", kubeconfig)

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			rlog.Errorf("KUBE-INIT Kubernetes out-of-cluster configuration problem: %s", err)
			os.Exit(1)
		}
	} else {
		rlog.Info("KUBE-INIT Connecting to kubernetes in-cluster")

		config, err = rest.InClusterConfig()
		if err != nil {
			rlog.Errorf("KUBE-INIT Kubernetes in-cluster configuration problem: %s", err)
			os.Exit(1)
		}
	}

	if _, err := os.Stat(KubeNamespaceFile); !os.IsNotExist(err) {
		res, err := ioutil.ReadFile(KubeNamespaceFile)
		if err != nil {
			rlog.Errorf("KUBE-INIT Cannot read namespace from %s: %s", KubeNamespaceFile, err)
			os.Exit(1)
		}

		KubernetesAntiopaNamespace = string(res)
	}
	if KubernetesAntiopaNamespace == "" {
		KubernetesAntiopaNamespace = os.Getenv("ANTIOPA_NAMESPACE")
	}
	if KubernetesAntiopaNamespace == "" {
		KubernetesAntiopaNamespace = KubeDefaultNamespace
	}

	KubernetesClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		rlog.Errorf("KUBE-INIT Kubernetes connection problem: %s", err)
		os.Exit(1)
	}

	rlog.Info("KUBE-INIT Successfully connected to kubernetes")

	// TODO: Запуск tiller
}

func KubeGetDeploymentImageName() string {

	res, err := KubernetesClient.AppsV1beta1().Deployments(KubernetesAntiopaNamespace).Get(KubeAntiopaDeploymentName, metav1.GetOptions{})

	if err != nil {
		rlog.Errorf("KUBE Cannot get antiopa deployment! %v", err)
		return ""
	}

	containersSpecs := res.Spec.Template.Spec.Containers

	for _, spec := range containersSpecs {
		if spec.Name == KubeAntiopaContainerName {
			return spec.Image
		}
	}

	return ""
}

func KubeGetPodImageName(podName string) string {
	res, err := KubernetesClient.CoreV1().Pods(KubernetesAntiopaNamespace).Get(podName, metav1.GetOptions{})

	if err != nil {
		rlog.Errorf("KUBE Cannot get info for pod %s! %v", podName, err)
		return ""
	}

	containersSpecs := res.Spec.Containers

	for _, spec := range containersSpecs {
		if spec.Name == KubeAntiopaContainerName {
			return spec.Image
		}
	}

	return ""
}

// KubeUpdateDeployment - меняет лейбл antiopaImageName на новый id образа antiopa
// тем самым заставляя kubernetes обновить Pod.
func KubeUpdateDeployment(imageId string) {
	deploymentsClient := KubernetesClient.AppsV1beta1().Deployments(KubernetesAntiopaNamespace)

	res,err := deploymentsClient.Get(KubeAntiopaDeploymentName, metav1.GetOptions{})

	if err != nil {
		rlog.Errorf("KUBE Cannot get antiopa deployment! %v", err)
		return
	}

	res.Spec.Template.Labels["antiopaImageName"] = NormalizeLabelValue(imageId)

	if _, err := deploymentsClient.Update(res); errors.IsConflict(err) {
		// Deployment is modified in the meanwhile, query the latest version
		// and modify the retrieved object.
		rlog.Errorf("KUBE Deployment manifest changed during update: %v", err)
	} else if err != nil {
		rlog.Errorf("KUBE Deployment update failed: %v", err)
	}

	return
}

var NonSafeCharsRegexp = regexp.MustCompile(`[^a-zA-Z0-9]`)

func NormalizeLabelValue(value string) string {
	newVal := NonSafeCharsRegexp.ReplaceAllLiteralString(value, "")
	labelLen := len(newVal)
	if labelLen > 63 {
		labelLen = 63
	}
	return newVal[:labelLen]
}
