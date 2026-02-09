package kubeclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var maintenanceAnnotations = [...]string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

func (c *Client) Start(ctx context.Context) error {
	c.logger.Info("starting kubeclient node informer")
	if err := c.startInformer(ctx); err != nil {
		return fmt.Errorf("failed to start node informer: %w", err)
	}
	return nil
}

func (c *Client) Stop() {
	c.logger.Info("stopping kubeclient")
	c.stopInformer()
	c.logger.Info("kubeclient stopped")
}

func (c *Client) IsMaintenanceMode() bool {
	return c.inMaintenanceMode.Load()
}

func (c *Client) startInformer(ctx context.Context) error {
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

func (c *Client) stopInformer() {
	if c.informerStopCh != nil {
		close(c.informerStopCh)
		c.logger.Info("node informer stopped")
	}
}
