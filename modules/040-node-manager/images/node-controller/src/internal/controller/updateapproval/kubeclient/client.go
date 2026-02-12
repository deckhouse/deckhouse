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

package kubeclient

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

type Client struct {
	Client client.Client
}

func (c Client) GetNodesForNodeGroup(ctx context.Context, ngName string) ([]corev1.Node, error) {
	return nodecommon.GetNodesForNodeGroup(ctx, c.Client, ngName)
}

func (c Client) GetConfigurationChecksums(ctx context.Context) (map[string]string, error) {
	return nodecommon.GetConfigurationChecksums(ctx, c.Client)
}

func (c Client) PatchNode(ctx context.Context, nodeName string, patch map[string]interface{}) error {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch for node %s: %w", nodeName, err)
	}
	if err := c.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("failed to patch node %s: %w", nodeName, err)
	}
	return nil
}

func (c Client) DeleteInstance(ctx context.Context, instanceName string) error {
	instance := &unstructured.Unstructured{}
	instance.SetAPIVersion("deckhouse.io/v1alpha1")
	instance.SetKind("Instance")
	instance.SetName(instanceName)

	if err := c.Client.Delete(ctx, instance, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete instance %s: %w", instanceName, err)
	}
	return nil
}
