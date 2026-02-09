package kubeclient

import (
	"fencing-agent/internal/lib/validators"
	"sync/atomic"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Config struct {
	QPS                  int           `env:"KUBERNETES_API_RPS" env-default:"10"`
	Burst                int           `env:"KUBERNETES_API_BURST" env-default:"100"`
	ConfigPath           string        `env:"KUBECONFIG" env-default:""` // equal to not set
	KubernetesAPITimeout time.Duration `env:"KUBERNETES_API_TIMEOUT" env-default:"10s"`
}

func (c *Config) Validate() error {
	if unaryErr := validators.ValidateRateLimit(c.QPS, c.Burst, "kubernetesAPI"); unaryErr != nil {
		return unaryErr
	}
	return nil
}

type Client struct {
	client            kubernetes.Interface
	logger            *log.Logger
	nodeName          string
	nodeGroup         string
	inMaintenanceMode atomic.Bool
	informerFactory   informers.SharedInformerFactory
	informerStopCh    chan struct{}
}

func New(cfg Config,
	logger *log.Logger,
	nodeName string,
	nodeGroup string) (*Client, error) {

	restConfig, err := buildConfig(cfg.ConfigPath)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = float32(cfg.QPS)
	restConfig.Burst = cfg.Burst
	restConfig.Timeout = cfg.KubernetesAPITimeout

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	client := &Client{
		client:    kubeClient,
		logger:    logger,
		nodeName:  nodeName,
		nodeGroup: nodeGroup,
	}

	return client, nil
}

// Reimplementation of clientcmd.buildConfig to avoid default warn message
func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		kubeconfig, err := rest.InClusterConfig()
		if err == nil {
			return kubeconfig, nil
		}
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
}
