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

package fencing

import (
	"context"
	"fmt"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	fencingConditionType = "Fencing"

	nodeStateSnapshot     = "fencing_node_state"
	agentPodsLeftSnapshot = "fencing_agent_pods_left"
)

// nodeStateAnnotations are the annotations that the agent writes on its Node.
var nodeStateAnnotations = []string{
	"node-manager.deckhouse.io/fencing-profile",
	"node-manager.deckhouse.io/fencing-cp-source",
}

// fencingNodeState tells which state of the agent is on a Node.
type fencingNodeState struct {
	Name        string
	Condition   bool
	Annotations bool
}

func applyFencingNodeStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	conditions, _, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}

	annotations := obj.GetAnnotations()

	return fencingNodeState{
		Name:        obj.GetName(),
		Condition:   slices.ContainsFunc(conditions, isFencingCondition),
		Annotations: slices.ContainsFunc(nodeStateAnnotations, func(key string) bool { _, ok := annotations[key]; return ok }),
	}, nil
}

// The hook only patches Nodes, so it has a queue of its own and does not wait for the hooks that write values.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/fencing-node-state",
	Kubernetes: []go_hook.KubernetesConfig{
		nodeGroupsBinding,
		{
			Name:       nodeStateSnapshot,
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyFencingNodeStateFilter,
		},
		// Helm deletes the DaemonSets of the agents in the background, and the agent pods stop later.
		// The deletion of the last agent pod starts the hook again.
		{
			Name:              agentPodsLeftSnapshot,
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{agentNamespace}}},
			LabelSelector:     &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: agentVersionLabel, Operator: metav1.LabelSelectorOpExists}}},
			FilterFunc:        nameFilter,
		},
	},
}, handleFencingNodeState)

// handleFencingNodeState removes the Fencing condition and the annotations of the agent from the Nodes when
// all node groups are Disabled and no agent pod is left. Then no agent runs, and the state that the agents
// left stays on the Nodes.
func handleFencingNodeState(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get(agentPodsLeftSnapshot)) > 0 {
		return nil
	}

	for ng, err := range sdkobjectpatch.SnapshotIter[nodeGroupFencing](input.Snapshots.Get(nodeGroupsSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %w", nodeGroupsSnapshot, err)
		}
		if normalizeMode(ng.Mode, input.Values.Get("global.deckhouseEdition").String()) != modeDisabled {
			return nil
		}
	}

	removeAnnotations := map[string]any{}
	for _, key := range nodeStateAnnotations {
		removeAnnotations[key] = nil
	}
	annotationsPatch := map[string]any{"metadata": map[string]any{"annotations": removeAnnotations}}

	for node, err := range sdkobjectpatch.SnapshotIter[fencingNodeState](input.Snapshots.Get(nodeStateSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %w", nodeStateSnapshot, err)
		}
		if node.Annotations {
			input.PatchCollector.PatchWithMerge(annotationsPatch, "v1", "Node", "", node.Name, object_patch.WithIgnoreMissingObject())
		}
		if node.Condition {
			input.PatchCollector.PatchWithMutatingFunc(removeFencingCondition, "v1", "Node", "", node.Name,
				object_patch.WithSubresource("/status"), object_patch.WithIgnoreMissingObject())
		}
	}

	return nil
}

func removeFencingCondition(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	obj = obj.DeepCopy()

	conditions, _, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}

	if err := unstructured.SetNestedSlice(obj.Object, slices.DeleteFunc(conditions, isFencingCondition), "status", "conditions"); err != nil {
		return nil, err
	}

	return obj, nil
}

func isFencingCondition(condition any) bool {
	fields, _ := condition.(map[string]any)
	return fields["type"] == fencingConditionType
}
