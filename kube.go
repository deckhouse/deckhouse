package main

import (
	"github.com/romana/rlog"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

var (
	KubernetesClient           *kubernetes.Clientset
	KubernetesAntiopaNamespace = "antiopa"
)

func InitKube() {
	rlog.Info("Init kube")

	var err error
	var config *rest.Config

	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); os.IsNotExist(err) {
		rlog.Info("Connecting to kubernetes out-of-cluster")

		var kubeconfig string
		if kubeconfig = os.Getenv("KUBECONFIG"); kubeconfig == "" {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
		rlog.Infof("Using kube config at %s", kubeconfig)

		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			rlog.Errorf("Kubernetes out-of-cluster configuration problem: %s", err)
			os.Exit(1)
		}
	} else {
		rlog.Info("Connecting to kubernetes in-cluster")

		config, err = rest.InClusterConfig()
		if err != nil {
			rlog.Errorf("Kubernetes in-cluster configuration problem: %s", err)
			os.Exit(1)
		}
	}

	nsFilename := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if _, err := os.Stat(nsFilename); os.IsNotExist(err) {
		res, err := ioutil.ReadFile("nsFilename")
		if err != nil {
			rlog.Errorf("Cannot read namespace from %s: %s", nsFilename, err)
			os.Exit(1)
		}

		KubernetesAntiopaNamespace = string(res)
	} else {
		KubernetesAntiopaNamespace = os.Getenv("ANTIOPA_NAMESPACE")
		if KubernetesAntiopaNamespace == "" {
			KubernetesAntiopaNamespace = "antiopa"
		}
	}

	KubernetesClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		rlog.Errorf("Kubernetes connection problem: %s", err)
		os.Exit(1)
	}

	rlog.Info("Successfully connected to kubernetes")

	// TODO: Запуск tiller
}
