package k8s

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func NewClient(devMode bool) *Client {
	var config *rest.Config
	var err error

	if devMode {

		var kubeconfig string
		kubeconfig, ok := os.LookupEnv("KUBECONFIG")
		if !ok {
			kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	return &Client{
		kubeClient: clientset,
	}
}
