package kubeapi

import (
	"context"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/lib/logger/sl"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var maintenanceAnnotations = [...]string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

type Provider struct {
	client    kubernetes.Interface
	logger    *log.Logger
	timeout   time.Duration
	nodeName  string
	nodeGroup string
}

func NewProvider(client kubernetes.Interface,
	logger *log.Logger,
	timeout time.Duration,
	nodeName string,
	nodeGroup string) *Provider {
	return &Provider{
		client:    client,
		logger:    logger,
		timeout:   timeout,
		nodeName:  nodeName,
		nodeGroup: nodeGroup,
	}
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
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	_, err := p.client.CoreV1().Nodes().List(ctx, v1meta.ListOptions{})
	if err != nil {
		p.logger.Debug("Kubernetes API is not available", sl.Err(err))
		return false
	}
	return true
}

func (p *Provider) IsMaintenanceMode(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

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
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

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
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

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

func (p *Provider) GetCurrentNodeIP(ctx context.Context, kubeClient kubernetes.Interface, nodeName string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, v1meta.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node=%s InternalIp for memberlist: %w", nodeName, err)
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("node %s has no InternalIP address", nodeName)
}
