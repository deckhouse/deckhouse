package kubeclient

import (
	"context"
	"fmt"
	"log/slog"

	v1 "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fencing-agent/internal/domain"
	"fencing-agent/internal/lib/logger/sl"
)

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
