package k8s

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	config "node-proxy-sidecar/internal/config"
)

func (c *Client) checkConnection(clientset *kubernetes.Clientset, host string) error {
	healthzURL := "/healthz"
	req := clientset.RESTClient().Get().AbsPath(healthzURL)
	result := req.Do(context.TODO())

	if result.Error() != nil {
		return fmt.Errorf("request to %s%s failed: %v", host, healthzURL, result.Error())
	}
	return nil
}

func (c *Client) NewClient(cfg config.Config) *Client {
	var clientset *kubernetes.Clientset

	switch cfg.AuthMode {
	case config.AuthCert:
		clientset = c.certAuthClient(cfg)
	case config.AuthDev:
		clientset = c.kubeConfigClient()
	default:
		log.Fatalln("Unsupported AuthMode")
	}

	if clientset == nil {
		log.Fatal("Failed to create Kubernetes client")
	}

	c.kubeClient = clientset
	return c
}

func (c *Client) certAuthClient(cfg config.Config) *kubernetes.Clientset {
	for _, host := range cfg.APIHosts {

		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			log.Fatalln("CA Certificate error", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		if err != nil {
			log.Fatalln("Client Certificate error", err)
		}

		kubeConfig := &rest.Config{
			Host: host,
			TLSClientConfig: rest.TLSClientConfig{
				CAData:   caCert,
				CAFile:   cfg.CACertPath,
				CertFile: cfg.CertPath,
				KeyFile:  cfg.KeyPath,
			},
		}

		clientset, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			log.Fatal("Kubernetes Client creation failed. ", err)
		}
		err = c.checkConnection(clientset, kubeConfig.Host)
		if err != nil {
			log.Error(err)
			continue
		}
		return clientset
	}
	return nil
}

func (c *Client) kubeConfigClient() *kubernetes.Clientset {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal("Kubernetes Client creation failed", err)
	}
	return clientset
}
