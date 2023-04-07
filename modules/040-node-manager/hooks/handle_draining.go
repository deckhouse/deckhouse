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

package hooks

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/draining",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_for_draining",
			WaitForSynchronization:       pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
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
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 30 * time.Second,
	},
}, dependency.WithExternalDependencies(handleDraining))

func drainFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var (
		isDraining     bool
		drainingSource string
	)
	if source, ok := node.Annotations["update.node.deckhouse.io/draining"]; ok {
		isDraining = true
		if source == "" {
			drainingSource = "bashible"
		} else {
			drainingSource = source
		}
	}

	return drainingNode{
		Name:           node.Name,
		IsDraining:     isDraining,
		DrainingSource: drainingSource,
		Unschedulable:  node.Spec.Unschedulable,
	}, nil
}

// Drain nodes: If node is marked for draining – drain it!
// all nodes in one node group drain concurrently. If we need to limit this behavior - put here some queue implementation
func handleDraining(input *go_hook.HookInput, dc dependency.Container) error {
	k8sCli, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	errOut := input.LogEntry.WriterLevel(logrus.WarnLevel)
	defer func(errOut *io.PipeWriter) {
		err := errOut.Close()
		if err != nil {
			input.LogEntry.Warningf("error closing logrus PipeWriter: %s", err)
		}
	}(errOut)

	drainHelper := drain.NewDrainer(k8sCli, errOut)
	drainHelper.Ctx = context.Background()

	var wg = &sync.WaitGroup{}
	drainingNodesC := make(chan drainedNodeRes, 1)

	snap := input.Snapshots["nodes_for_draining"]
	for _, s := range snap {
		dNode := s.(drainingNode)
		if !dNode.IsDraining {
			continue
		}
		cordonNode := &corev1.Node{
			TypeMeta: v1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: dNode.Name,
			},
			Spec: corev1.NodeSpec{Unschedulable: dNode.Unschedulable},
		}
		err := drain.RunCordonOrUncordon(drainHelper, cordonNode, true)
		if err != nil {
			input.LogEntry.Errorf("Cordon node '%s' failed: %s", dNode.Name, err)
			continue
		}

		wg.Add(1)
		go func(nodeName, drainingSource string) {
			defer wg.Done()
			err = drain.RunNodeDrain(drainHelper, nodeName)
			drainingNodesC <- drainedNodeRes{NodeName: nodeName, DrainingSource: drainingSource, Err: err}
		}(dNode.Name, dNode.DrainingSource)
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
		input.PatchCollector.MergePatch(newDrainAnnotationPatch(drainedNode.DrainingSource), "v1", "Node", "", drainedNode.NodeName)
	}

	return nil
}

func newDrainAnnotationPatch(source string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"update.node.deckhouse.io/draining": nil,
				"update.node.deckhouse.io/drained":  source,
			},
		},
	}
}

type drainingNode struct {
	Name           string
	IsDraining     bool
	DrainingSource string
	Unschedulable  bool
}

type drainedNodeRes struct {
	NodeName       string
	DrainingSource string
	Err            error
}
