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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	drainingAnnotationKey = "update.node.deckhouse.io/draining"
	drainedAnnotationKey  = "update.node.deckhouse.io/drained"
	nodeGroupLabel        = "node.deckhouse.io/group"
	defaultDrainTimeout   = 10 * time.Minute
)

var nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/draining",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes_for_draining",
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      nodeGroupLabel,
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
		ngName         string
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

	if drainingSource == "" && drainedSource == "" {
		return nil, nil
	}

	ngName = node.Labels[nodeGroupLabel]

	return drainingNode{
		Name:           node.Name,
		NodeGroupName:  ngName,
		DrainingSource: drainingSource,
		DrainedSource:  drainedSource,
		Unschedulable:  node.Spec.Unschedulable,
	}, nil
}

// Drain nodes: If node is marked for draining – drain it!
// all nodes in one node group drain concurrently. If we need to limit this behavior - put here some queue implementation
func handleDraining(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sCli, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	drainTimeoutCache := make(map[string]time.Duration)
	wg := &sync.WaitGroup{}
	drainingNodesC := make(chan drainedNodeRes, 1)

	dNodes := input.Snapshots.Get("nodes_for_draining")
	for dNode, err := range sdkobjectpatch.SnapshotIter[drainingNode](dNodes) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes_for_draining' snapshots: %w", err)
		}

		drainTimeout, exists := drainTimeoutCache[dNode.NodeGroupName]
		if !exists {
			drainTimeout = getDrainTimeout(input, k8sCli, dNode.NodeGroupName)
			drainTimeoutCache[dNode.NodeGroupName] = drainTimeout
		}

		drainHelper := drain.NewDrainer(drain.HelperConfig{Client: k8sCli, Timeout: &drainTimeout})
		drainHelper.Ctx = context.Background()
		if !dNode.isDraining() {
			// If the node became schedulable, but 'drained' annotation is still on it, remove the obsolete annotation
			if !dNode.Unschedulable && dNode.DrainedSource == "user" {
				input.PatchCollector.PatchWithMerge(removeDrainedAnnotation, "v1", "Node", "", dNode.Name)
			}
			continue
		}

		// If the node is marked for draining while is has been drained, remove the 'drained' annotation
		if dNode.DrainedSource == "user" {
			input.PatchCollector.PatchWithMerge(removeDrainedAnnotation, "v1", "Node", "", dNode.Name)
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
			input.Logger.Error("Cordon node failed", slog.String("name", dNode.Name), log.Err(err))
			continue
		}

		wg.Add(1)
		go func(node drainingNode) {
			input.Logger.Info("Node draining: started", "node", node)
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
			input.Logger.Info("Node draining: finished", "node", node)
			wg.Done()
		}(dNode)
	}

	go func() {
		wg.Wait()
		close(drainingNodesC)
	}()

	input.MetricsCollector.Expire("d8_node_draining")
	var shouldIgnoreErr bool
	for drainedNode := range drainingNodesC {
		if drainedNode.Err != nil {
			input.Logger.Error("node drain failed", slog.String("name", drainedNode.NodeName), log.Err(drainedNode.Err))
			shouldIgnoreErr = errors.Is(drainedNode.Err, drain.ErrDrainTimeout)
			event := drainedNode.buildEvent()
			input.PatchCollector.CreateOrUpdate(event)
			input.MetricsCollector.Set("d8_node_draining", 1, map[string]string{"node": drainedNode.NodeName, "message": drainedNode.Err.Error()})
			if shouldIgnoreErr {
				input.Logger.Error("node drain error skipped", slog.String("name", drainedNode.NodeName), log.Err(drainedNode.Err))
			} else {
				continue
			}
		}
		input.PatchCollector.PatchWithMerge(newDrainedAnnotationPatch(drainedNode.DrainingSource), "v1", "Node", "", drainedNode.NodeName)
	}

	return nil
}

func getDrainTimeout(input *go_hook.HookInput, client k8s.Client, ngName string) time.Duration {
	nodeGroups, err := client.Dynamic().Resource(nodeGroupResource).Namespace("").List(context.TODO(), v1.ListOptions{})
	nodeGroup := new(ngv1.NodeGroup)

	if err != nil {
		input.Logger.Error("Failed to list node groups")
		return defaultDrainTimeout
	}

	for _, group := range nodeGroups.Items {
		err := sdk.FromUnstructured(&group, nodeGroup)
		if err != nil {
			input.Logger.Error("Error marshling NodeGroup", "ngName", ngName, "error", err)
			return defaultDrainTimeout
		}
		groupName := nodeGroup.Name
		if groupName != ngName {
			continue
		}

		timeoutValue := nodeGroup.Spec.NodeDrainTimeoutSecond
		if timeoutValue != nil {
			drainTimeout := time.Duration(*timeoutValue) * time.Second
			return drainTimeout
		}
	}

	input.Logger.Info("Node group not found, use defaultDrainTimeout", "ngName", ngName)
	return defaultDrainTimeout
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

var removeDrainedAnnotation = map[string]interface{}{
	"metadata": map[string]interface{}{
		"annotations": map[string]interface{}{
			drainedAnnotationKey: nil,
		},
	},
}

type drainingNode struct {
	Name           string
	NodeGroupName  string
	DrainingSource string
	DrainedSource  string
	Unschedulable  bool
}

func (dn drainingNode) isDraining() bool {
	return dn.DrainingSource != ""
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
