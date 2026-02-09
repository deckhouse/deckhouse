package kubeclient

import (
	"context"
	"fencing-agent/internal/lib/logger/sl"

	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) ShouldFeed(ctx context.Context) bool {
	_, err := c.client.CoreV1().Nodes().List(ctx, v1meta.ListOptions{})
	if err != nil {
		c.logger.Debug("kubernetes API is not available", sl.Err(err))
		return false
	}
	return true
}
