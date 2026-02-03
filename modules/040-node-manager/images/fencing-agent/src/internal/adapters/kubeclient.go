package adapters

import (
	"context"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/helper/logger/sl"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var maintenanceAnnotations = [...]string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

type Provider struct {
	client    kubernetes.Interface
	logger    *log.Logger
	nodeName  string
	nodeGroup string
}

func NewProvider(kubeconfigPath string,
	timeout time.Duration,
	qps float32, burst int,
	logger *log.Logger,
	nodeName string,
	nodeGroup string) (*Provider, error) {

	var restConfig *rest.Config
	var kubeClient *kubernetes.Clientset
	var err error

	restConfig, err = buildConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = qps
	restConfig.Burst = burst
	restConfig.Timeout = timeout

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &Provider{
		client:    kubeClient,
		logger:    logger,
		nodeName:  nodeName,
		nodeGroup: nodeGroup,
	}, nil
}

func (p *Provider) GetNodes(ctx context.Context) ([]domain.Node, error) {
	labelSelector := fmt.Sprintf("node.deckhouse.io/group=%s", p.nodeGroup)
	p.logger.Debug("Get nodes", slog.String("labelSelector", labelSelector))
	nodes, err := p.client.CoreV1().Nodes().List(ctx, v1meta.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		p.logger.Warn("Failed to get nodes from kubeapi", sl.Err(err))
		return nil, err
	}
	p.logger.Debug("Get nodes", slog.Int("count", len(nodes.Items)))
	dNodes := make([]domain.Node, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		dNode := domain.NewNode(node.Name)
		dNode.LastSeen = time.Now()
		for _, ip := range node.Status.Addresses {
			if ip.Type == "InternalIP" {
				dNode.Addresses[domain.InterfaceName] = ip.Address
				break
			}
		}
		dNodes = append(dNodes, dNode)
	}
	return dNodes, nil
}

func (p *Provider) IsAvailable(ctx context.Context) bool {
	_, err := p.client.CoreV1().Nodes().List(ctx, v1meta.ListOptions{})
	if err != nil {
		p.logger.Debug("Kubernetes API is not available", sl.Err(err))
		return false
	}
	return true
}

func (p *Provider) IsMaintenanceMode(ctx context.Context) (bool, error) {
	node, err := p.client.CoreV1().Nodes().Get(ctx, p.nodeName, v1meta.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get node %s: %w", p.nodeName, err)
	}
	for _, annotation := range maintenanceAnnotations {
		if _, exists := node.Annotations[annotation]; exists {
			p.logger.Info("Maintenance mode is on",
				slog.String("node", p.nodeName),
				slog.String("annotation", annotation))
			return true, nil
		}
	}
	return false, nil
}

func (p *Provider) SetNodeLabel(ctx context.Context, key, value string) error {
	patch := []byte(fmt.Sprintf(
		`{"metadata":{"labels":{%q:%q}}}`,
		key, value,
	))

	_, err := p.client.CoreV1().Nodes().
		Patch(ctx, p.nodeName, types.MergePatchType, patch, v1meta.PatchOptions{})

	if err != nil {
		return fmt.Errorf("failed to patch node %s labels: %w", p.nodeName, err)
	}

	p.logger.Info("Node label set",
		slog.String("node", p.nodeName),
		slog.String("label", key),
		slog.String("value", value))
	return nil
}

func (p *Provider) RemoveNodeLabel(ctx context.Context, key string) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, key))

	_, err := p.client.CoreV1().Nodes().
		Patch(ctx, p.nodeName, types.MergePatchType, patch, v1meta.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node %s labels: %w", p.nodeName, err)
	}

	p.logger.Info("Node label removed",
		slog.String("node", p.nodeName),
		slog.String("label", key))
	return nil
}

func (p *Provider) GetCurrentNodeIP(ctx context.Context) (string, error) {
	node, err := p.client.CoreV1().Nodes().Get(ctx, p.nodeName, v1meta.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node=%s InternalIp for memberlist: %w", p.nodeName, err)
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("node %s has no InternalIP address", p.nodeName)
}

func NewClient(kubeconfigPath string, timeout time.Duration, qps float32, burst int) (*kubernetes.Clientset, error) {
	var restConfig *rest.Config
	var kubeClient *kubernetes.Clientset
	var err error

	restConfig, err = buildConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = qps
	restConfig.Burst = burst
	restConfig.Timeout = timeout

	kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return kubeClient, nil
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
