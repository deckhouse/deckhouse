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

package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"gopkg.in/robfig/cron.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	defragStateCMName      = "d8-control-plane-manager-etcd-defrag"
	defragStateCMNamespace = "kube-system"
	defragLastSlotKey      = "lastHandledCronSlot"

	clusterIsBootstrappedPath = "global.clusterIsBootstrapped"

	// defragGracePeriod is the maximum allowed delay between a cron slot firing and
	// the hook handling it. If the slot is older than this (e.g. Deckhouse was down),
	// we record it as handled and skip to avoid stale defrag runs.
	defragGracePeriod = 5 * time.Minute
)

// defragNow is a hook for tests to inject a fixed time.
var defragNow = func() time.Time {
	return time.Now().UTC()
}

type defragStateCM struct {
	LastHandledCronSlot string
}

type defragMasterNode struct {
	Name string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/etcd_defrag",
	Schedule: []go_hook.ScheduleConfig{
		{
			Crontab: "* * * * *",
			Name:    "etcd-defrag-tick",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes_defrag",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterDefragNode,
		},
		{
			Name:       "arbiter_nodes_defrag",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node.deckhouse.io/etcd-arbiter": "",
				},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterDefragNode,
		},
		{
			Name:       "defrag_state_cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{defragStateCMNamespace},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{defragStateCMName}},
			FilterFunc:   filterDefragStateCM,
		},
	},
}, handleSpawnEtcdDefragCPO)

func filterDefragNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return defragMasterNode{Name: obj.GetName()}, nil
}

func filterDefragStateCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	if err := sdk.FromUnstructured(obj, &cm); err != nil {
		return nil, fmt.Errorf("parse defrag state ConfigMap: %w", err)
	}
	return defragStateCM{LastHandledCronSlot: cm.Data[defragLastSlotKey]}, nil
}

func handleSpawnEtcdDefragCPO(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get(etcdDefragEnabledInternalPath).Bool() {
		return nil
	}
	if !input.Values.Get(clusterIsBootstrappedPath).Bool() {
		return nil
	}

	cronSpec := input.Values.Get(etcdDefragScheduleInternalPath).String()
	if cronSpec == "" {
		return nil
	}

	sched, err := cron.Parse("TZ=UTC " + cronSpec)
	if err != nil {
		return fmt.Errorf("parse etcd defrag cron schedule %q: %w", cronSpec, err)
	}

	currentSlot := defragNow().Truncate(time.Minute)

	var lastHandled time.Time
	cmSnaps := input.Snapshots.Get("defrag_state_cm")
	if len(cmSnaps) > 0 {
		cm, err := sdkobjectpatch.UnmarshalToStruct[defragStateCM](input.Snapshots, "defrag_state_cm")
		if err != nil {
			return fmt.Errorf("unmarshal defrag state ConfigMap: %w", err)
		}
		if len(cm) > 0 && cm[0].LastHandledCronSlot != "" {
			parsed, err := time.Parse(time.RFC3339, cm[0].LastHandledCronSlot)
			if err != nil {
				return fmt.Errorf("parse lastHandledCronSlot %q: %w", cm[0].LastHandledCronSlot, err)
			}
			lastHandled = parsed.UTC()
		}
	}

	nextSlot := sched.Next(lastHandled)
	if currentSlot.Before(nextSlot) {
		return nil
	}

	// Grace period: if the slot is older than defragGracePeriod (e.g. Deckhouse was down),
	// record it as handled and skip — running a stale defrag is pointless.
	if currentSlot.Sub(nextSlot) > defragGracePeriod {
		stateData := map[string]string{defragLastSlotKey: nextSlot.Format(time.RFC3339)}
		input.PatchCollector.CreateOrUpdate(buildDefragStateCM(stateData))
		return nil
	}

	// Collect all etcd nodes (masters + arbiters), deduplicated by name.
	seen := make(map[string]struct{})
	var nodeNames []string

	for node, err := range sdkobjectpatch.SnapshotIter[defragMasterNode](input.Snapshots.Get("master_nodes_defrag")) {
		if err != nil {
			return fmt.Errorf("iterate master_nodes_defrag: %w", err)
		}
		if _, exists := seen[node.Name]; !exists {
			seen[node.Name] = struct{}{}
			nodeNames = append(nodeNames, node.Name)
		}
	}
	for node, err := range sdkobjectpatch.SnapshotIter[defragMasterNode](input.Snapshots.Get("arbiter_nodes_defrag")) {
		if err != nil {
			return fmt.Errorf("iterate arbiter_nodes_defrag: %w", err)
		}
		if _, exists := seen[node.Name]; !exists {
			seen[node.Name] = struct{}{}
			nodeNames = append(nodeNames, node.Name)
		}
	}

	if len(nodeNames) == 0 {
		return nil
	}

	slotSuffix := nextSlot.Format("060102-1504")
	for _, nodeName := range nodeNames {
		cpo := buildDefragCPO(nodeName, slotSuffix)
		input.PatchCollector.CreateIfNotExists(cpo)
	}

	stateData := map[string]string{
		defragLastSlotKey: currentSlot.Format(time.RFC3339),
	}
	cm := buildDefragStateCM(stateData)
	input.PatchCollector.CreateOrUpdate(cm)

	return nil
}

func buildDefragCPO(nodeName, slotSuffix string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "control-plane.deckhouse.io/v1alpha1",
			"kind":       "ControlPlaneOperation",
			"metadata": map[string]interface{}{
				"name":      "etcd-defrag-" + nodeName + "-" + slotSuffix,
				"namespace": defragStateCMNamespace,
				"labels": map[string]interface{}{
					"control-plane.deckhouse.io/node":      nodeName,
					"control-plane.deckhouse.io/component": "etcd",
					"heritage":                             "deckhouse",
					"module":                               "control-plane-manager",
				},
			},
			"spec": map[string]interface{}{
				"nodeName":  nodeName,
				"component": "Etcd",
				"steps":     []interface{}{"DefragEtcd"},
				"approved":  true,
			},
		},
	}
}

func buildDefragStateCM(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defragStateCMName,
			Namespace: defragStateCMNamespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "control-plane-manager",
			},
		},
		Data: data,
	}
}
