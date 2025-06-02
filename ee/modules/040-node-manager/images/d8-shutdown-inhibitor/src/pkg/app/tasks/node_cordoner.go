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

package tasks

import (
	"context"
	"fmt"

	"d8_shutdown_inhibitor/pkg/kubernetes"
)

// NodeCordoner waits for shutdown signal and cordons the node.
type NodeCordoner struct {
	NodeName         string
	ShutdownSignalCh <-chan struct{}
}

func (n *NodeCordoner) Name() string {
	return "nodeCordoner"
}

func (n *NodeCordoner) Run(ctx context.Context, _ chan error) {
	// Stage 1. Wait for shutdown.
	fmt.Printf("nodeCordoner: wait for PrepareForShutdown signal or power key press\n")
	select {
	case <-ctx.Done():
		fmt.Printf("nodeCordoner: stop on global exit\n")
		// Return now, cordon is not needed in case of the global stop.
		return
	case <-n.ShutdownSignalCh:
		fmt.Printf("nodeCordoner: catch prepare shutdown signal, cordon node\n")
	}

	kubectl := kubernetes.NewDefaultKubectl()
	output, err := kubectl.Cordon(n.NodeName)
	if err != nil {
		fmt.Printf("nodeCordoner: fail to cordon node: %v\n, output: %s\n", err, string(output))
	}
}
