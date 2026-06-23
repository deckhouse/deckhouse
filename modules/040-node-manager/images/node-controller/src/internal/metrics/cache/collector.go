/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
)

type Collector struct {
	client client.Client
}

func NewCollector(c client.Client) *Collector {
	return &Collector{
		client: c,
	}
}

func (c *Collector) Start(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		c.collect(ctx)

		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
		}
	}
}

func (c *Collector) collect(ctx context.Context) {
	_ = c.collectTyped(ctx, "Node", "", &corev1.NodeList{})

	_ = c.collectTyped(ctx, "Secret", "", &corev1.SecretList{})

	_ = c.collectTyped(ctx, "NodeGroup", "deckhouse.io", &deckhousev1.NodeGroupList{})

	_ = c.collectTyped(ctx, "Machine", "machine.sapcloud.io", &mcmv1alpha1.MachineList{})

	_ = c.collectTyped(ctx, "Machine", "cluster.x-k8s.io", &capiv1beta2.MachineList{})

	_ = c.collectTyped(ctx, "Instance", "deckhouse.io", &deckhousev1alpha2.InstanceList{})

	_ = c.collectUnstructured(
		ctx,
		"MachineDeployment",
		"machine.sapcloud.io",
		schema.GroupVersionKind{
			Group:   "machine.sapcloud.io",
			Version: "v1alpha1",
			Kind:    "MachineDeploymentList",
		},
	)

	_ = c.collectUnstructured(
		ctx,
		"MachineDeployment",
		"cluster.x-k8s.io",
		schema.GroupVersionKind{
			Group:   "cluster.x-k8s.io",
			Version: "v1beta2",
			Kind:    "MachineDeploymentList",
		},
	)
}

func (c *Collector) collectTyped(
	ctx context.Context,
	kind string,
	apiGroup string,
	list client.ObjectList,
) error {
	if err := c.client.List(ctx, list); err != nil {
		return err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}

	cacheObjectsGauge.WithLabelValues(kind, apiGroup).Set(float64(len(items)))
	return nil
}

func (c *Collector) collectUnstructured(
	ctx context.Context,
	kind string,
	apiGroup string,
	gvk schema.GroupVersionKind,
) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := c.client.List(ctx, list); err != nil {
		return err
	}

	cacheObjectsGauge.WithLabelValues(
		kind,
		apiGroup,
	).Set(float64(len(list.Items)))

	return nil
}
