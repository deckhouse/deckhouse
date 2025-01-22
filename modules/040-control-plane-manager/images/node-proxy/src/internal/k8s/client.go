package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := req.Do(ctx)

	if result.Error() != nil {
		return fmt.Errorf("request to %s%s failed: %v", host, healthzURL, result.Error())
	}
	return nil
}

func (c *Client) NewClient(cfg config.Config) (*Client, error) {
	var (
		clientset *kubernetes.Clientset
		err       error
	)

	switch cfg.AuthMode {
	case config.AuthCert:
		clientset, err = c.certAuthClient(cfg)
		if err != nil {
			return nil, err
		}
	case config.AuthDev:
		clientset, err = c.kubeConfigClient()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported auth mode: %s", cfg.AuthMode)
	}

	if clientset == nil {
		return nil, fmt.Errorf("failed to create k8s client")
	}

	c.client = clientset
	return c, nil
}

func (c *Client) certAuthClient(cfg config.Config) (*kubernetes.Clientset, error) {
	for _, host := range cfg.APIHosts {
		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %v", err)
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
			return nil, fmt.Errorf("create k8s client: %v", err)
		}

		err = c.checkConnection(clientset, kubeConfig.Host)
		if err != nil {
			log.Warnf("connection check failed for host %s: %v", host, err)
			continue
		}
		return clientset, nil
	}
	return nil, fmt.Errorf("no available hosts for cert-based auth")
}

func (c *Client) kubeConfigClient() (*kubernetes.Clientset, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("create k8s client: %v", err)
	}
	return clientset, nil
}
