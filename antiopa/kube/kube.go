package kube

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/romana/rlog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1beta1 "k8s.io/client-go/kubernetes/typed/apps/v1beta1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1alpha1 "k8s.io/client-go/kubernetes/typed/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
)

const (
	KubeTokenFilePath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	KubeNamespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	DefaultNamespace      = "antiopa"
	AntiopaDeploymentName = "antiopa"
	AntiopaContainerName  = "antiopa"
	AntiopaSecret         = "antiopa"
	AntiopaConfigMap      = "antiopa"
)

var (
	KubernetesClient           Client
	KubernetesAntiopaNamespace string
)

type Client interface {
	CoreV1() corev1.CoreV1Interface
	AppsV1beta1() appsv1beta1.AppsV1beta1Interface
	RbacV1alpha1() rbacv1alpha1.RbacV1alpha1Interface
	RbacV1beta1() rbacv1beta1.RbacV1beta1Interface
}

func IsRunningOutOfKubeCluster() bool {
	_, err := os.Stat(KubeTokenFilePath)
	return os.IsNotExist(err)
}

// InitKube - инициализация kubernetes клиента
// Можно подключить изнутри, а можно на основе .kube директории
func InitKube() {
	rlog.Info("KUBE Init Kubernetes client")

	var err error
	var config *rest.Config

	if IsRunningOutOfKubeCluster() {
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

	if _, err := os.Stat(KubeNamespaceFilePath); !os.IsNotExist(err) {
		res, err := ioutil.ReadFile(KubeNamespaceFilePath)
		if err != nil {
			rlog.Errorf("KUBE-INIT Cannot read namespace from %s: %s", KubeNamespaceFilePath, err)
			os.Exit(1)
		}

		KubernetesAntiopaNamespace = string(res)
	}
	if KubernetesAntiopaNamespace == "" {
		KubernetesAntiopaNamespace = os.Getenv("ANTIOPA_NAMESPACE")
	}
	if KubernetesAntiopaNamespace == "" {
		KubernetesAntiopaNamespace = DefaultNamespace
	}

	KubernetesClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		rlog.Errorf("KUBE-INIT Kubernetes connection problem: %s", err)
		os.Exit(1)
	}

	rlog.Info("KUBE-INIT Successfully connected to kubernetes")
}

func KubeGetDeploymentImageName() string {
	res, err := KubernetesClient.AppsV1beta1().Deployments(KubernetesAntiopaNamespace).Get(AntiopaDeploymentName, metav1.GetOptions{})

	if err != nil {
		rlog.Errorf("KUBE Cannot get antiopa deployment! %v", err)
		return ""
	}

	containersSpecs := res.Spec.Template.Spec.Containers

	for _, spec := range containersSpecs {
		if spec.Name == AntiopaContainerName {
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
		if spec.Name == AntiopaContainerName {
			return spec.Image
		}
	}

	return ""
}

// KubeUpdateDeployment - меняет лейбл antiopaImageName на новый id образа antiopa
// тем самым заставляя kubernetes обновить Pod.
func KubeUpdateDeployment(imageId string) error {
	deploymentsClient := KubernetesClient.AppsV1beta1().Deployments(KubernetesAntiopaNamespace)

	res, err := deploymentsClient.Get(AntiopaDeploymentName, metav1.GetOptions{})

	if err != nil {
		return fmt.Errorf("Cannot get antiopa deployment! %v", err)
	}

	res.Spec.Template.Labels["antiopaImageId"] = NormalizeLabelValue(imageId)

	if _, err := deploymentsClient.Update(res); errors.IsConflict(err) {
		// Deployment is modified in the meanwhile, query the latest version
		// and modify the retrieved object.
		return fmt.Errorf("Manifest changed during update: %v", err)
	} else if err != nil {
		return err
	}

	return nil
}

var NonSafeCharsRegexp = regexp.MustCompile(`[^a-zA-Z0-9]`)

func NormalizeLabelValue(value string) string {
	newVal := NonSafeCharsRegexp.ReplaceAllLiteralString(value, "_")
	labelLen := len(newVal)
	if labelLen > 63 {
		labelLen = 63
	}
	return newVal[:labelLen]
}

func GetConfigMap() (*v1.ConfigMap, error) {
	configMap, err := KubernetesClient.CoreV1().ConfigMaps(KubernetesAntiopaNamespace).Get(AntiopaConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Cannot get ConfigMap %s from namespace %s: %s", AntiopaConfigMap, KubernetesAntiopaNamespace, err)
	}

	return configMap, nil
}
