/*
Copyright 2026 Flant JSC

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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const vcpWorkerNodeGroupName = "worker"

var vcpNodeGroupGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "nodegroups",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Mirror of createMasterNodeGroup for a virtual control plane tenant:
	// seed a default static `worker` NodeGroup so a fresh tenant renders a manual-bootstrap secret out of the box.
	// It never patches, so an operator can edit the NG freely.
	Queue: "/modules/node-manager/create-vcp-worker-ng",
	// Ensure crds hook has order 5, create the node group after it.
	OnStartup: &go_hook.OrderedConfig{Order: 6},
}, dependency.WithExternalDependencies(createVCPWorkerNodeGroup))

func getDefaultVCPWorkerNg() (*unstructured.Unstructured, error) {
	ng := map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": vcpWorkerNodeGroupName,
		},
		// Manual, not the Automatic default: a one-worker tenant would cordon its only node and
		// evict kube-dns with nowhere to put it. No NeedDrainNode guard covers a group named worker.
		"spec": map[string]interface{}{
			"nodeType": "Static",
			"disruptions": map[string]interface{}{
				"approvalMode": "Manual",
			},
		},
	}
	return sdk.ToUnstructured(&ng)
}

// GET first: CreateIfNotExists still sends the CREATE, so the webhook rejects it when its backend has
// nowhere to run, wedging the startup phase ahead of helm. No snapshot - onStartup precedes its sync.
func createVCPWorkerNodeGroup(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	if !nestedControlPlane(input) {
		return nil
	}

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %w", err)
	}

	_, err = kubeClient.Dynamic().Resource(vcpNodeGroupGVR).Get(ctx, vcpWorkerNodeGroupName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get NodeGroup %q: %w", vcpWorkerNodeGroupName, err)
	}

	ng, err := getDefaultVCPWorkerNg()
	if err != nil {
		return err
	}

	input.PatchCollector.CreateIfNotExists(ng)

	return nil
}
