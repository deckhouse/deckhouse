package kubeclient

import (
	"context"
	"fmt"
	"log/slog"

	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
