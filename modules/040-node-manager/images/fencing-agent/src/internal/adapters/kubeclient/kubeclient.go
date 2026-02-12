package kubeclient

import (
	"context"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/lib/logger/sl"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/lib/validators"
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

var maintenanceAnnotations = [...]string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
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
	nodeGroup string,
) (*Client, error) {
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

func (c *Client) SetNodeLabel(ctx context.Context, key, value string) error {
	patch := []byte(fmt.Sprintf(
		`{"metadata":{"labels":{%q:%q}}}`,
		key, value,
	))

	_, err := c.client.CoreV1().Nodes().
		Patch(ctx, c.nodeName, types.MergePatchType, patch, v1meta.PatchOptions{})

	if err != nil {
		return fmt.Errorf("failed to patch node %s labels: %w", c.nodeName, err)
	}

	c.logger.Info("node label set",
		slog.String("node", c.nodeName),
		slog.String("label", key),
		slog.String("value", value))
	return nil
}

func (c *Client) RemoveNodeLabel(ctx context.Context, key string) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, key))

	_, err := c.client.CoreV1().Nodes().
		Patch(ctx, c.nodeName, types.MergePatchType, patch, v1meta.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node %s labels: %w", c.nodeName, err)
	}

	c.logger.Info("node label removed",
		slog.String("node", c.nodeName),
		slog.String("label", key))
	return nil
}

func (c *Client) GetNodesIP(ctx context.Context) ([]string, error) {
	labelSelector := fmt.Sprintf("node.deckhouse.io/group=%s", c.nodeGroup)

	c.logger.Debug("get nodes", slog.String("label_selector", labelSelector))

	nodes, err := c.client.CoreV1().Nodes().List(ctx, v1meta.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		c.logger.Warn("failed to get nodes from kubeapi", sl.Err(err))
		return nil, err
	}

	ips := make([]string, 0, len(nodes.Items))

	for _, node := range nodes.Items {
		for _, ip := range node.Status.Addresses {
			if ip.Type == domain.InterfaceName {
				ips = append(ips, ip.Address)
				break
			}
		}
	}

	return ips, nil
}

func (c *Client) GetCurrentNodeIP(ctx context.Context) (string, error) {
	node, err := c.client.CoreV1().Nodes().Get(ctx, c.nodeName, v1meta.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node=%s InternalIp for memberlist: %w", c.nodeName, err)
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("node %s has no InternalIP address", c.nodeName)
}

func (c *Client) IsMaintenanceMode() bool {
	return c.inMaintenanceMode.Load()
}

func (c *Client) StartInformer(ctx context.Context) error {
	c.informerFactory = informers.NewSharedInformerFactory(c.client, 30*time.Second)

	nodeInformer := c.informerFactory.Core().V1().Nodes().Informer()

	_, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*v1.Node)
			if !ok {
				c.logger.Warn("failed to cast object to Node in AddFunc")
				return
			}
			if node.Name == c.nodeName {
				c.checkMaintenanceAnnotations(node)
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			node, ok := newObj.(*v1.Node)
			if !ok {
				c.logger.Warn("failed to cast object to Node in UpdateFunc")
				return
			}
			if node.Name == c.nodeName {
				c.checkMaintenanceAnnotations(node)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// If our node is deleted, we're likely shutting down anyway
			node, ok := obj.(*v1.Node)
			if !ok {
				c.logger.Warn("failed to cast object to Node in DeleteFunc")
				return
			}
			if node.Name == c.nodeName {
				c.logger.Warn("current node deleted from cluster")
			}
		},
	})

	if err != nil {
		return fmt.Errorf("failed to add event handler to node informer: %w", err)
	}

	c.informerStopCh = make(chan struct{})

	c.informerFactory.Start(c.informerStopCh)

	if !cache.WaitForCacheSync(ctx.Done(), nodeInformer.HasSynced) {
		return fmt.Errorf("failed to sync node informer cache")
	}
	c.logger.Info("node informer cache synced successfully")
	return nil
}

func (c *Client) checkMaintenanceAnnotations(node *v1.Node) {
	hasAnnotation := false
	var foundAnnotation string

	for _, annotation := range maintenanceAnnotations {
		if _, exists := node.Annotations[annotation]; exists {
			hasAnnotation = true
			foundAnnotation = annotation
			break
		}
	}

	oldValue := c.inMaintenanceMode.Swap(hasAnnotation)

	if oldValue != hasAnnotation {
		if hasAnnotation {
			c.logger.Info("maintenance mode detected",
				slog.String("node", c.nodeName),
				slog.String("annotation", foundAnnotation))
		} else {
			c.logger.Info("maintenance mode cleared",
				slog.String("node", c.nodeName))
		}
	}
}

func (c *Client) StopInformer() {
	if c.informerStopCh != nil {
		close(c.informerStopCh)
		c.logger.Info("node informer stopped")
	}
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
