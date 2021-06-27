/*
Copyright 2021 Flant CJSC

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
	"encoding/json"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"
)

var (
	waitForSync = false
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/draining",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "nodes_for_draining",
			WaitForSynchronization: &waitForSync,
			ApiVersion:             "v1",
			Kind:                   "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: drainFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "draining_schedule",
			Crontab: "* * * * *",
		},
	},
}, dependency.WithExternalDependencies(handleDraining))

func drainFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var isDraining bool
	if _, ok := node.Annotations["update.node.deckhouse.io/draining"]; ok {
		isDraining = true
	}

	return drainingNode{Name: node.Name, IsDraining: isDraining}, nil
}

// Drain nodes: If node is marked for draining – drain it!
// all nodes in one node group drain concurrently. If we need to limit this behavior - put here some queue implementation
func handleDraining(input *go_hook.HookInput, dc dependency.Container) error {
	k8sCli, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	drainHelper := drain.NewDrainer(k8sCli)

	var wg = &sync.WaitGroup{}
	drainingNodesC := make(chan drainedNodeRes, 1)

	snap := input.Snapshots["nodes_for_draining"]
	for _, s := range snap {
		node := s.(drainingNode)
		if !node.IsDraining {
			continue
		}

		wg.Add(1)
		go func(nodeName string) {
			defer wg.Done()
			err = drain.RunNodeDrain(drainHelper, nodeName)
			drainingNodesC <- drainedNodeRes{Name: nodeName, Err: err}
		}(node.Name)
	}

	go func() {
		wg.Wait()
		close(drainingNodesC)
	}()

	for drainedNode := range drainingNodesC {
		if drainedNode.Err != nil {
			input.LogEntry.Errorf("node drain failed: %s", drainedNode.Err)
			continue
		}
		err = input.ObjectPatcher.MergePatchObject(drainAnnotationsPatch, "v1", "Node", "", drainedNode.Name, "")
		if err != nil {
			input.LogEntry.Errorf("node drain patch failed: %s", err)
			continue
		}

	}

	return nil
}

var (
	drainAnnotations = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"update.node.deckhouse.io/draining": nil,
				"update.node.deckhouse.io/drained":  "",
			},
		},
	}

	drainAnnotationsPatch, _ = json.Marshal(drainAnnotations)
)

type drainingNode struct {
	Name       string
	IsDraining bool
}

type drainedNodeRes struct {
	Name string
	Err  error
}
