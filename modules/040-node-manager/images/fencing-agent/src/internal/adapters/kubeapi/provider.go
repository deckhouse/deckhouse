package kubeapi

import (
	"context"
	"fencing-agent/internal/core/domain"
	"fmt"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var maintenanceAnnotations = [...]string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

type Provider struct {
	client    kubernetes.Interface
	logger    *zap.Logger
	timeout   time.Duration
	nodeName  string
	nodeGroup string
}

func NewProvider(client kubernetes.Interface,
	logger *zap.Logger,
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
	p.logger.Debug("Get nodes", zap.String("labelSelector", labelSelector))
	nodes, err := p.client.CoreV1().Nodes().List(ctx, v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		// TODO logging
		return nil, err
	}
	p.logger.Debug("Get nodes", zap.Int("count", len(nodes.Items)))
	dNodes := make([]domain.Node, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		dNode := domain.NewNode(node.Name)
		dNode.LastSeen = time.Now()
		for _, ip := range node.Status.Addresses {
			if ip.Type == "InternalIP" {
				dNode.Addresses["eth0"] = ip.Address
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

	_, err := p.client.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		p.logger.Debug("Kubernetes API is not available", zap.Error(err))
		return false
	}
	return true
}

func (p *Provider) IsMaintenanceMode(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	node, err := p.client.CoreV1().Nodes().Get(ctx, p.nodeName, v1.GetOptions{})
	if err != nil {
		// TODO logging
		return false, fmt.Errorf("failed to get node %s: %w", p.nodeName, err)
	}
	for _, annotation := range maintenanceAnnotations {
		if _, exists := node.Annotations[annotation]; exists {
			p.logger.Info("Maintenance mode is on",
				zap.String("node", p.nodeName),
				zap.String("annotation", annotation))
			return true, nil
		}
	}
	return false, nil
}
