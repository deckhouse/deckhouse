/*
Copyright 2021 Flant JSC

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

package nodes

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const uninitializedTaintKey = "node.cloudprovider.kubernetes.io/uninitialized"

func RegisterWaitToBecomeReadyHook() bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	}, dependency.WithExternalDependencies(waitForAllMasterNodesToBecomeInitialized))
}

func isAllMasterNodesInitialized(_ context.Context, input *go_hook.HookInput, dc dependency.Container) (bool, error) {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		input.Logger.Error(err.Error())
		return false, err
	}
	masterNodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/control-plane="})
	if err != nil {
		input.Logger.Error(err.Error())
		return false, err
	}

	for _, node := range masterNodes.Items {
		for _, taint := range node.Spec.Taints {
			if taint.Key == uninitializedTaintKey {
				return false, fmt.Errorf("master has taint %s", uninitializedTaintKey)
			}
		}
	}
	return true, nil
}

func waitForAllMasterNodesToBecomeInitialized(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	var lastErr error

	err := wait.PollUntilContextTimeout(context.TODO(), time.Second, 120*time.Second, false, func(_ context.Context) (bool, error) {
		input.Logger.Info("waiting for master nodes to become initialized by cloud provider")
		ok, err := isAllMasterNodesInitialized(ctx, input, dc)

		if err != nil {
			lastErr = err
			return false, err
		}

		return ok, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for master nodes. last error: %v", lastErr)
	}

	return nil
}
