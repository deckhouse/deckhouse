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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Mirror of createMasterNodeGroup for a virtual control plane tenant:
	// seed a default static `worker` NodeGroup so a fresh tenant renders a manual-bootstrap secret out of the box.
	// It is idempotent (CreateIfNotExists) and never patches, so an operator can edit the NG freely.
	Queue: "/modules/node-manager/create-vcp-worker-ng",
	// Ensure crds hook has order 5, create the node group after it.
	OnStartup: &go_hook.OrderedConfig{Order: 6},
}, createVCPWorkerNodeGroup)

func getDefaultVCPWorkerNg() (*unstructured.Unstructured, error) {
	ng := map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": "worker",
		},
		"spec": map[string]interface{}{
			"nodeType": "Static",
		},
	}
	return sdk.ToUnstructured(&ng)
}

func createVCPWorkerNodeGroup(_ context.Context, input *go_hook.HookInput) error {
	if !nestedControlPlane(input) {
		return nil
	}

	ng, err := getDefaultVCPWorkerNg()
	if err != nil {
		return err
	}

	// Do not patch the node group if it already exists, to avoid conflicts with user changes.
	input.PatchCollector.CreateIfNotExists(ng)

	return nil
}
