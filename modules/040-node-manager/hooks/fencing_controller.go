/*
Copyright 2024 Flant JSC

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
	"log/slog"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	nodesSnapshot            = "nodes"
	fencingControllerTimeout = time.Duration(60) * time.Second
)

var maintenanceAnnotations = []string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/fencing",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "run_node_fencing_every_minute",
			Crontab: "* * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       nodesSnapshot,
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "node-manager.deckhouse.io/fencing-enabled",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: fencingControllerNodeFilter,
		},
	},
}, dependency.WithExternalDependencies(fencingControllerHandler))

type fencingControllerNodeResult struct {
	Name          string
	NodeGroupName string
}

func fencingControllerNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var res fencingControllerNodeResult

	for _, annotation := range maintenanceAnnotations {
		_, annotationExists := obj.GetAnnotations()[annotation]
		if annotationExists {
			return nil, nil
		}
	}

	res.Name = obj.GetName()
	return res, nil
}

func fencingControllerHandler(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	if len(input.Snapshots.Get(nodesSnapshot)) == 0 {
		// No nodes with enabled fencing -> nothing to do
		return nil
	}

	// kubeclient to get node leases and get and delete pods
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		input.Logger.Error(err.Error())
		return err
	}

	// make map with nodes to kill
	nodesToKill := set.New()
	for node, err := range sdkobjectpatch.SnapshotIter[fencingControllerNodeResult](input.Snapshots.Get(nodesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshots: %w", err)
		}

		nodeLease, err := kubeClient.CoordinationV1().Leases("kube-node-lease").Get(context.TODO(), node.Name, metav1.GetOptions{})
		if err != nil {
			input.Logger.Error("Can't get node lease", log.Err(err))
			continue
		}

		if time.Since(nodeLease.Spec.RenewTime.Time) > fencingControllerTimeout {
			input.Logger.Warn(
				"Node lease is expired",
				slog.String("name", node.Name),
				slog.String("current time", time.Now().String()),
				slog.String("node lease time", nodeLease.Spec.RenewTime.Time.String()),
			)
			nodesToKill.Add(node.Name)
		}
	}

	nodeToKillCount := nodesToKill.Size()
	if nodeToKillCount == 0 {
		// nothing to kill -> skip
		return nil
	}

	input.Logger.Warn("Going to kill nodes", slog.Int("count", nodeToKillCount))

	// kill nodes
	for _, node := range nodesToKill.Slice() {
		input.Logger.Warn("Delete all pods from node", slog.String("name", node))
		podsToDelete, err := kubeClient.CoreV1().Pods("").List(
			context.TODO(),
			metav1.ListOptions{
				FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node}).String(),
			},
		)
		if err != nil {
			input.Logger.Error("Can't list pods", log.Err(err))
			continue
		}

		GracePeriodSeconds := int64(0)
		for _, pod := range podsToDelete.Items {
			input.Logger.Warn("Delete pod in namespace on node", slog.String("name", pod.Name), slog.String("namespace", pod.Namespace), slog.String("node", pod.Spec.NodeName))
			err = kubeClient.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &GracePeriodSeconds,
			})
			if err != nil {
				input.Logger.Error("Can't delete pod", slog.String("name", pod.Name), log.Err(err))
			}
		}

		input.Logger.Warn("Delete node", slog.String("name", node))
		input.PatchCollector.DeleteInBackground("v1", "Node", "", node)
	}

	return nil
}
