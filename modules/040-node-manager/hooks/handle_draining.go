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
	"os"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"
)

const (
	drainingAnnotationKey = "update.node.deckhouse.io/draining"
	drainedAnnotationKey  = "update.node.deckhouse.io/drained"
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
		drainingSource string
		drainedSource  string
	)
	if source, ok := node.Annotations[drainingAnnotationKey]; ok {
		// keep backward compatibility
		if source == "" {
			drainingSource = "bashible"
		} else {
			drainingSource = source
		}
	}

	if source, ok := node.Annotations[drainedAnnotationKey]; ok {
		// keep backward compatibility
		if source == "" {
			drainedSource = "bashible"
		} else {
			drainedSource = source
		}
	}

	return drainingNode{
		Name:           node.Name,
		DrainingSource: drainingSource,
		DrainedSource:  drainedSource,
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
		if !dNode.isDraining() {
			// If the node became schedulable, but 'drained' annotation is still on it, remove the obsolete annotation
			if !dNode.Unschedulable && dNode.DrainedSource == "user" {
				input.PatchCollector.MergePatch(removeDrainedAnnotation, "v1", "Node", "", dNode.Name)
			}
			continue
		}

		// If the node is marked for draining while is has been drained, remove the 'drained' annotation
		if dNode.DrainedSource == "user" {
			input.PatchCollector.MergePatch(removeDrainedAnnotation, "v1", "Node", "", dNode.Name)
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
		go func(node drainingNode) {
			if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
				if node.Name == "foo-2" {
					drainHelper.PodSelector = "a: b._c"
				}
			}

			err = drain.RunNodeDrain(drainHelper, node.Name)
			drainingNodesC <- drainedNodeRes{
				NodeName:       node.Name,
				DrainingSource: node.DrainingSource,
				Err:            err,
			}
			wg.Done()
		}(dNode)
	}

	go func() {
		wg.Wait()
		close(drainingNodesC)
	}()

	input.MetricsCollector.Expire("d8_node_draining")
	for drainedNode := range drainingNodesC {
		if drainedNode.Err != nil {
			input.LogEntry.Errorf("node %q drain failed: %s", drainedNode.NodeName, drainedNode.Err)
			event := drainedNode.buildEvent()
			input.PatchCollector.Create(event, object_patch.UpdateIfExists())
			input.MetricsCollector.Set("d8_node_draining", 1, map[string]string{"node": drainedNode.NodeName, "message": drainedNode.Err.Error()})
			continue
		}
		input.PatchCollector.MergePatch(newDrainedAnnotationPatch(drainedNode.DrainingSource), "v1", "Node", "", drainedNode.NodeName)
	}

	return nil
}

func newDrainedAnnotationPatch(source string) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				drainingAnnotationKey: nil,
				drainedAnnotationKey:  source,
			},
		},
	}
}

var (
	removeDrainedAnnotation = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				drainedAnnotationKey: nil,
			},
		},
	}
)

type drainingNode struct {
	Name           string
	DrainingSource string
	DrainedSource  string
	Unschedulable  bool
}

func (dn drainingNode) isDraining() bool {
	return dn.DrainingSource != ""
}

func (dn drainingNode) isDrained() bool {
	return dn.DrainedSource != ""
}

type drainedNodeRes struct {
	NodeName       string
	DrainingSource string
	Err            error
}

func (dr drainedNodeRes) buildEvent() *eventsv1.Event {
	return &eventsv1.Event{
		TypeMeta: v1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			// Namespace field has to be filled - event will not be created without it
			// and we have to set 'default' value here for linking this event with a Node object, which is global
			Namespace:    "default",
			GenerateName: "node-" + dr.NodeName + "-",
		},
		Regarding: corev1.ObjectReference{
			Kind: "Node",
			Name: dr.NodeName,
			// nodeName is used for both .name and .uid fields intentionally as putting a real node uid
			// has proven to have some side effects like missing events when describing objects using kubectl versions 1.23.x
			UID:        types.UID(dr.NodeName),
			APIVersion: "deckhouse.io/v1",
		},
		Reason:              "DrainFailed",
		Note:                dr.Err.Error(),
		Type:                "Warning",
		EventTime:           v1.MicroTime{Time: time.Now()},
		Action:              "Binding",
		ReportingInstance:   "deckhouse",
		ReportingController: "deckhouse",
	}
}
