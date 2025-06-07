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

package main

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetes "k8s.io/client-go/kubernetes"
)

const labelKey = "ingress-nginx-controller.deckhouse.io/need-hostwithfailover-cleanup"

// HasFailoverLabelOnNode checks if the current node has a failover label with value true.
// This is used to decide whether iptables rules should be preserved on termination.
func HasFailoverLabelOnNode(ctx context.Context, client kubernetes.Interface, nodeName string) (bool, error) {
	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("cannot get node %s: %w", nodeName, err)
	}

	val, ok := node.Labels[labelKey]
	return ok && val == "true", nil
}

func RemoveFailoverLabel(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				labelKey: nil,
			},
		},
	}
	rawPatch, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = client.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, rawPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %w", err)
	}

	return nil
}
