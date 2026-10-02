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

// CAPI-side counterpart of modules/040-node-manager/hooks/yc_delete_preemptible_instances.go
// (an MCM-only hook, a no-op on CAPI-managed NodeGroups): rotates preemptible YandexMachines
// before Yandex.Cloud force-stops the underlying VM at 24h.

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	// Preemptible instances are forcibly stopped by Yandex.Cloud after 24 hours
	// https://cloud.yandex.com/en-ru/docs/compute/concepts/preemptible-vm
	hookExecutionSchedule = 15 * time.Minute
	// we'll delete Machines that are almost ready to be terminated by the cloud provider
	durationThresholdForDeletion = 24*time.Hour - 4*time.Hour

	// we won't delete any Machines if it would violate overall Node readiness of a given NodeGroup
	nodeGroupReadinessRatio = 0.9

	capyMachineNamespace = "d8-cloud-instance-manager"
	// bare "node-group" (no prefix) is the label CAPI copies from Machine down to YandexMachine.
	capyNodeGroupLabel = "node-group"
)

type capyYandexMachine struct {
	Name              string
	Terminating       bool
	Preemptible       bool
	Ready             bool
	NodeGroup         string
	CreationTimestamp metav1.Time
}

type capyNodeGroupStatus struct {
	Name  string
	Nodes int64
	Ready int64
}

func applyYandexMachineFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// absent field and explicit false both mean "not preemptible" - ok==false is not an error.
	preemptible, ok, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "preemptible")
	if err != nil {
		return nil, fmt.Errorf("can't access field \"spec.preemptible\" of YandexMachine %q: %s", obj.GetName(), err)
	}

	ready, _, err := unstructured.NestedBool(obj.UnstructuredContent(), "status", "ready")
	if err != nil {
		return nil, fmt.Errorf("can't access field \"status.ready\" of YandexMachine %q: %s", obj.GetName(), err)
	}

	return &capyYandexMachine{
		Name:              obj.GetName(),
		Terminating:       obj.GetDeletionTimestamp() != nil,
		Preemptible:       ok && preemptible,
		Ready:             ready,
		NodeGroup:         obj.GetLabels()[capyNodeGroupLabel],
		CreationTimestamp: obj.GetCreationTimestamp(),
	}, nil
}

func applyCapyNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	icKind, icExists, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "cloudInstances", "classReference", "kind")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"spec.cloudInstances.classReference.kind\" in a NodeGroup %s: %s", obj.GetName(), err)
	}

	if !icExists || (icKind != "YandexInstanceClass") {
		return nil, nil
	}

	nodeCountRaw, nodeCountExists, err := unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "status", "nodes")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"status.nodes\" in a NodeGroup %s: %s", obj.GetName(), err)
	}
	readyNodeCountRaw, readyNodeCountExists, err := unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "status", "ready")
	if err != nil {
		return nil, fmt.Errorf("cannot access \"status.ready\" in a NodeGroup %s: %s", obj.GetName(), err)
	}

	if !nodeCountExists || !readyNodeCountExists {
		return nil, nil
	}

	var nodeCount, readyNodeCount int64
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		nodeCount = int64(nodeCountRaw.(float64))
		readyNodeCount = int64(readyNodeCountRaw.(float64))
	} else {
		nodeCount = nodeCountRaw.(int64)
		readyNodeCount = readyNodeCountRaw.(int64)
	}

	if (nodeCount < 0) || (readyNodeCount < 0) {
		return nil, nil
	}

	return &capyNodeGroupStatus{
		Name:  obj.GetName(),
		Nodes: nodeCount,
		Ready: readyNodeCount,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	AllowFailure: true,
	Queue:        "/modules/cloud-provider-yandex/preemptible-rotation",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name: "every-15",
			// string formatting is ugly, but serves a purpose of referencing an important constant
			Crontab: fmt.Sprintf("0/%.0f * * * *", hookExecutionSchedule.Minutes()),
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "yandexmachines",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "infrastructure.cluster.x-k8s.io/v1alpha1",
			Kind:                "YandexMachine",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{capyMachineNamespace},
				},
			},
			FilterFunc: applyYandexMachineFilter,
		},
		{
			Name:                "nodegroupstatuses",
			ExecuteHookOnEvents: go_hook.Bool(false),
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "NodeGroup",
			FilterFunc:          applyCapyNodeGroupFilter,
		},
	},
}, deleteCapyPreemptibleMachines)

func deleteCapyPreemptibleMachines(_ context.Context, input *go_hook.HookInput) error {
	var (
		timeNow                        = time.Now().UTC()
		nodeGroupNameToNodeGroupStatus = make(map[string]*capyNodeGroupStatus)
		candidatesByNodeGroup          = make(map[string][]*capyYandexMachine)
	)

	for ngStatus, err := range sdkobjectpatch.SnapshotIter[capyNodeGroupStatus](input.Snapshots.Get("nodegroupstatuses")) {
		if err != nil {
			return fmt.Errorf("failed to assert to NodeGroupStatus: failed to iterate over 'nodegroupstatuses' snapshot: %w", err)
		}

		nodeGroupNameToNodeGroupStatus[ngStatus.Name] = &ngStatus
	}

	for ym, err := range sdkobjectpatch.SnapshotIter[capyYandexMachine](input.Snapshots.Get("yandexmachines")) {
		if err != nil {
			return fmt.Errorf("failed to assert to YandexMachine: failed to iterate over 'yandexmachines' snapshot: %w", err)
		}

		if ym.Terminating {
			continue
		}

		if !ym.Preemptible {
			continue
		}

		// not yet up: capy might still be mid-create, must not be force-deleted.
		if !ym.Ready {
			continue
		}

		// skip young YandexMachines
		if ym.CreationTimestamp.Time.Add(durationThresholdForDeletion).After(timeNow) {
			continue
		}

		// skip YandexMachines in NodeGroups that violate NodeGroup readiness ratio
		ngStatus, ok := nodeGroupNameToNodeGroupStatus[ym.NodeGroup]
		if !ok {
			continue
		}
		if (float64(ngStatus.Ready) / float64(ngStatus.Nodes)) < nodeGroupReadinessRatio {
			continue
		}

		candidatesByNodeGroup[ym.NodeGroup] = append(candidatesByNodeGroup[ym.NodeGroup], &ym)
	}

	for nodeGroup, candidates := range candidatesByNodeGroup {
		for _, m := range getCapyMachinesToDelete(candidates) {
			input.Logger.Info("deleting preemptible YandexMachine's Machine for rotation",
				slog.String("machine", m.Name), slog.String("node_group", nodeGroup), slog.Time("created_at", m.CreationTimestamp.Time))
			input.PatchCollector.Delete("cluster.x-k8s.io/v1beta2", "Machine", capyMachineNamespace, m.Name)
		}
	}

	return nil
}

// getCapyMachinesToDelete batches per NodeGroup (candidates are already grouped by caller) so one
// large NodeGroup's rotation never starves a smaller NodeGroup's single candidate in the same run.
func getCapyMachinesToDelete(candidates []*capyYandexMachine) []*capyYandexMachine {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreationTimestamp.Before(&candidates[j].CreationTimestamp)
	})

	// take 10% of old YandexMachines
	batch := len(candidates) / 10
	if batch == 0 {
		batch = 1
	}

	machinesToDelete := make([]*capyYandexMachine, 0, batch)
	for _, m := range candidates {
		if len(machinesToDelete) < batch {
			machinesToDelete = append(machinesToDelete, m)
		} else {
			break
		}
	}

	return machinesToDelete
}
