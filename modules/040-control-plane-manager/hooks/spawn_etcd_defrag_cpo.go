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
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/robfig/cron.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
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

// defragNow is overridden in tests to inject a fixed time.
var defragNow = time.Now

type defragCPN struct {
	Name string
	UID  k8stypes.UID
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
			Name:       "control_plane_nodes_defrag",
			ApiVersion: "control-plane.deckhouse.io/v1alpha1",
			Kind:       "ControlPlaneNode",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{defragStateCMNamespace},
				},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterDefragCPN,
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
			NameSelector:        &types.NameSelector{MatchNames: []string{defragStateCMName}},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterDefragStateCM,
		},
	},
}, handleSpawnEtcdDefragCPO)

func filterDefragCPN(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return defragCPN{
		Name: obj.GetName(),
		UID:  obj.GetUID(),
	}, nil
}

func filterDefragStateCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	if err := sdk.FromUnstructured(obj, &cm); err != nil {
		return nil, fmt.Errorf("parse defrag state ConfigMap: %w", err)
	}
	return cm.Data[defragLastSlotKey], nil
}

func handleSpawnEtcdDefragCPO(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get(etcdDefragEnabledInternalPath).Bool() {
		input.Logger.Debug("etcd defrag disabled, skipping")
		return nil
	}
	if !input.Values.Get(clusterIsBootstrappedPath).Bool() {
		input.Logger.Debug("cluster not bootstrapped yet, skipping")
		return nil
	}

	cronSpec := input.Values.Get(etcdDefragScheduleInternalPath).String()
	sched, err := cron.Parse("TZ=UTC " + cronSpec)
	if err != nil {
		input.Logger.Warn("etcd defrag cronSchedule is invalid, skipping tick", "cronSchedule", cronSpec, "err", err)
		return nil
	}

	currentSlot := defragNow().UTC().Truncate(time.Minute)

	var lastHandled time.Time
	for slot, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get("defrag_state_cm")) {
		if err != nil {
			return fmt.Errorf("unmarshal defrag state ConfigMap: %w", err)
		}
		if slot != "" {
			parsed, err := time.Parse(time.RFC3339, slot)
			if err != nil {
				return fmt.Errorf("parse lastHandledCronSlot %q: %w", slot, err)
			}
			lastHandled = parsed.UTC()
		}
	}

	// First install: no state CM yet. Record the current time so the first real
	// defrag fires at the next scheduled slot, not immediately after deploy.
	if lastHandled.IsZero() {
		input.Logger.Info("etcd defrag: first run, initializing state; CPOs will be created at the next scheduled slot")
		input.PatchCollector.CreateOrUpdate(buildDefragStateCM(map[string]string{
			defragLastSlotKey: currentSlot.Format(time.RFC3339),
		}))
		return nil
	}

	nextSlot := sched.Next(lastHandled)
	input.Logger.Debug("etcd defrag cron check",
		"currentSlot", currentSlot.Format(time.RFC3339),
		"lastHandled", lastHandled.Format(time.RFC3339),
		"nextSlot", nextSlot.Format(time.RFC3339),
	)

	if currentSlot.Before(nextSlot) {
		return nil
	}

	// Grace period: if the slot is older than defragGracePeriod (e.g. Deckhouse was down),
	// record it as handled and skip — running a stale defrag is pointless.
	if currentSlot.Sub(nextSlot) > defragGracePeriod {
		input.Logger.Warn("etcd defrag: slot missed by more than grace period, skipping",
			"nextSlot", nextSlot.Format(time.RFC3339),
			"delay", currentSlot.Sub(nextSlot).String(),
			"gracePeriod", defragGracePeriod.String(),
		)
		stateData := map[string]string{defragLastSlotKey: currentSlot.Format(time.RFC3339)}
		input.PatchCollector.CreateOrUpdate(buildDefragStateCM(stateData))
		return nil
	}

	// Collect all etcd nodes (masters + arbiters) from ControlPlaneNode snapshots.
	var nodeNames []string
	cpnUIDs := make(map[string]k8stypes.UID)
	for cpn, err := range sdkobjectpatch.SnapshotIter[defragCPN](input.Snapshots.Get("control_plane_nodes_defrag")) {
		if err != nil {
			return fmt.Errorf("iterate control_plane_nodes_defrag: %w", err)
		}
		nodeNames = append(nodeNames, cpn.Name)
		cpnUIDs[cpn.Name] = cpn.UID
	}

	if len(nodeNames) == 0 {
		input.Logger.Warn("etcd defrag: no etcd nodes found, skipping")
		return nil
	}

	input.Logger.Info("etcd defrag: spawning CPOs", "nodes", nodeNames, "slot", nextSlot.Format(time.RFC3339))

	for _, nodeName := range nodeNames {
		name := etcdDefragCPOName(nextSlot, nodeName)
		input.PatchCollector.CreateIfNotExists(buildDefragCPO(name, nodeName, nextSlot, cpnUIDs[nodeName]))
		input.Logger.Info("etcd defrag: CPO created", "name", name, "node", nodeName)
	}

	stateData := map[string]string{
		defragLastSlotKey: currentSlot.Format(time.RFC3339),
	}
	input.PatchCollector.CreateOrUpdate(buildDefragStateCM(stateData))

	return nil
}

func etcdDefragCPOName(slotTime time.Time, nodeName string) string {
	slot := sha256.Sum256([]byte(slotTime.Format(time.RFC3339)))
	name := sha256.Sum256([]byte(nodeName))
	return fmt.Sprintf("etcd-defrag-%x-%x", slot[:3], name[:3])
}

func buildDefragCPO(cpoName, nodeName string, slotTime time.Time, cpnUID k8stypes.UID) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "control-plane.deckhouse.io/v1alpha1",
		"kind":       "ControlPlaneOperation",
		"metadata": map[string]interface{}{
			"name":      cpoName,
			"namespace": defragStateCMNamespace,
			"labels": map[string]interface{}{
				"control-plane.deckhouse.io/node":      nodeName,
				"control-plane.deckhouse.io/component": "etcd",
				"control-plane.deckhouse.io/slot":      slotTime.Format("060102-1504"),
				"heritage":                             "deckhouse",
				"module":                               "control-plane-manager",
			},
		},
		"spec": map[string]interface{}{
			"nodeName":  nodeName,
			"component": "Etcd",
			"steps":     []interface{}{"DefragEtcd", "WaitPodReady"},
			"approved":  false,
		},
	}

	if cpnUID != "" {
		obj["metadata"].(map[string]interface{})["ownerReferences"] = []interface{}{
			map[string]interface{}{
				"apiVersion":         "control-plane.deckhouse.io/v1alpha1",
				"kind":               "ControlPlaneNode",
				"name":               nodeName,
				"uid":                string(cpnUID),
				"controller":         true,
				"blockOwnerDeletion": false,
			},
		}
	}

	return &unstructured.Unstructured{Object: obj}
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
