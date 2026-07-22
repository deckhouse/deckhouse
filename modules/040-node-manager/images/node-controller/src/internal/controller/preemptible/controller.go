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

// Package preemptible proactively rotates the oldest preemptible Yandex.Cloud nodes
// before the cloud provider force-stops them. Yandex terminates preemptible VMs after
// at most 24h, so once a node's age crosses the 20h (24h-4h) window this controller
// deletes its MCM Machine (machine.sapcloud.io/v1alpha1); MCM then recreates a fresh
// node. To preserve availability it deletes at most 10% of eligible Machines per tick
// and skips any NodeGroup below a 0.9 ready ratio.
//
// This replaces the shell-operator hook hooks/yc_delete_preemptible_instances.go. The
// hook was schedule-driven (crontab "0/15 * * * *") with only passive Kubernetes
// bindings, so this controller reproduces it with a 15-minute ticker rather than a
// reactive watch — NodeGroup/Node/Machine events must NOT trigger it.
//
// Scope: MCM only (machine.sapcloud.io/v1alpha1 YandexMachineClass), matching the hook
// 1:1. CAPI-backed Yandex NodeGroups (cluster.x-k8s.io) are not handled — the CAPI
// Yandex provider does not support preemptible instances at all yet, so there is
// nothing to rotate. See the migration TODO.
package preemptible

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	nodeGroupLabel          = "node.deckhouse.io/group"
	yandexInstanceClassKind = "YandexInstanceClass"
	yandexMachineClassKind  = "YandexMachineClass"

	// Preemptible instances are forcibly stopped by Yandex.Cloud after 24h; delete
	// Machines that are almost ready to be terminated by the cloud provider.
	// https://cloud.yandex.com/en-ru/docs/compute/concepts/preemptible-vm
	durationThresholdForDeletion = 24*time.Hour - 4*time.Hour

	// Don't delete any Machines if it would violate overall Node readiness of a NodeGroup.
	nodeGroupReadinessRatio = 0.9

	tickInterval = 15 * time.Minute
)

var (
	machineGVK            = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "Machine"}
	yandexMachineClassGVK = schema.GroupVersionKind{Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "YandexMachineClass"}
)

func newMachineList() *unstructured.UnstructuredList {
	l := &unstructured.UnstructuredList{}
	l.SetGroupVersionKind(machineGVK.GroupVersion().WithKind("MachineList"))
	return l
}

func newYandexMachineClassList() *unstructured.UnstructuredList {
	l := &unstructured.UnstructuredList{}
	l.SetGroupVersionKind(yandexMachineClassGVK.GroupVersion().WithKind("YandexMachineClassList"))
	return l
}

func init() {
	register.RegisterController("yandex-preemptible-cleanup", &deckhousev1.NodeGroup{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Drop all primary (NodeGroup) events: the hook was schedule-only, so reactive
	// triggers would rotate nodes more often than every 15 minutes.
	w.WithEventFilter(predicate.NewPredicateFuncs(func(client.Object) bool { return false }))

	// 15-minute ticker reproducing the hook's crontab "0/15 * * * *". Raw sources are
	// not filtered by WithEventFilter, so this is the only thing that enqueues.
	w.WatchesRawSource(source.Func(func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
		go func() {
			ticker := time.NewTicker(tickInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: "yandex-preemptible-cleanup"}})
				}
			}
		}()
		return nil
	}))
}

type nodeInfo struct {
	group             string
	creationTimestamp time.Time
}

type ngStatus struct {
	nodes int32
	ready int32
}

type deletionCandidate struct {
	name                  string
	nodeCreationTimestamp time.Time
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	timeNow := time.Now().UTC()

	preemptibleClasses, err := r.preemptibleMachineClasses(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	// No preemptible YandexMachineClass → nothing to rotate (parity with the hook's
	// early return and MCM-only scope).
	if len(preemptibleClasses) == 0 {
		return ctrl.Result{}, nil
	}

	nodes, err := r.nodesByName(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	ngStatuses, err := r.yandexNodeGroupStatuses(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	candidates, err := r.collectDeletionCandidates(ctx, timeNow, preemptibleClasses, nodes, ngStatuses)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(candidates) == 0 {
		return ctrl.Result{}, nil
	}

	for _, name := range machinesToDelete(candidates) {
		if err := r.deleteMachine(ctx, name); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("deleted preemptible machine ahead of cloud eviction", "machine", name)
	}

	return ctrl.Result{}, nil
}

// preemptibleMachineClasses returns the set of YandexMachineClass names that provision
// preemptible instances (spec.schedulingPolicy.preemptible == true).
func (r *Reconciler) preemptibleMachineClasses(ctx context.Context) (map[string]struct{}, error) {
	list := newYandexMachineClassList()
	if err := r.Client.List(ctx, list, client.InNamespace(nodecommon.MachineNamespace)); err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		mc := &list.Items[i]
		preemptible, ok, err := unstructured.NestedBool(mc.Object, "spec", "schedulingPolicy", "preemptible")
		if err != nil {
			return nil, fmt.Errorf("can't access spec.schedulingPolicy.preemptible of YandexMachineClass %q: %w", mc.GetName(), err)
		}
		if ok && preemptible {
			result[mc.GetName()] = struct{}{}
		}
	}
	return result, nil
}

// nodesByName maps node name to its NodeGroup and creation time. Nodes without the
// group label are skipped (parity with the hook's applyNodeFilter nil result).
func (r *Reconciler) nodesByName(ctx context.Context) (map[string]nodeInfo, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return nil, err
	}

	byName := make(map[string]nodeInfo, len(nodeList.Items))
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		group, ok := node.Labels[nodeGroupLabel]
		if !ok {
			continue
		}
		byName[node.Name] = nodeInfo{group: group, creationTimestamp: node.CreationTimestamp.Time}
	}
	return byName, nil
}

// yandexNodeGroupStatuses maps NodeGroup name to its node/ready counts for Yandex
// (YandexInstanceClass) NodeGroups only.
func (r *Reconciler) yandexNodeGroupStatuses(ctx context.Context) (map[string]ngStatus, error) {
	ngList := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return nil, err
	}

	byName := make(map[string]ngStatus, len(ngList.Items))
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if ng.Spec.CloudInstances == nil || ng.Spec.CloudInstances.ClassReference.Kind != yandexInstanceClassKind {
			continue
		}
		if ng.Status.Nodes < 0 || ng.Status.Ready < 0 {
			continue
		}
		byName[ng.Name] = ngStatus{nodes: ng.Status.Nodes, ready: ng.Status.Ready}
	}
	return byName, nil
}

// collectDeletionCandidates walks the Machines and keeps the non-terminating,
// preemptible, old-enough ones whose NodeGroup still holds the readiness ratio.
func (r *Reconciler) collectDeletionCandidates(
	ctx context.Context,
	timeNow time.Time,
	preemptibleClasses map[string]struct{},
	nodes map[string]nodeInfo,
	ngStatuses map[string]ngStatus,
) ([]deletionCandidate, error) {
	machineList := newMachineList()
	if err := r.Client.List(ctx, machineList, client.InNamespace(nodecommon.MachineNamespace)); err != nil {
		return nil, err
	}

	candidates := make([]deletionCandidate, 0, len(machineList.Items))
	for i := range machineList.Items {
		m := &machineList.Items[i]

		if m.GetDeletionTimestamp() != nil {
			continue
		}

		classKind, _, err := unstructured.NestedString(m.Object, "spec", "class", "kind")
		if err != nil {
			return nil, fmt.Errorf("can't access spec.class.kind of Machine %q: %w", m.GetName(), err)
		}
		if classKind != yandexMachineClassKind {
			continue
		}

		className, _, err := unstructured.NestedString(m.Object, "spec", "class", "name")
		if err != nil {
			return nil, fmt.Errorf("can't access spec.class.name of Machine %q: %w", m.GetName(), err)
		}
		if _, ok := preemptibleClasses[className]; !ok {
			continue
		}

		// The hook keys the node lookup on the Machine name (MCM names the node after
		// its Machine); a Machine with no matching Node is skipped.
		node, ok := nodes[m.GetName()]
		if !ok {
			continue
		}

		// Skip young Machines: only rotate once the node is close to the cloud's 24h limit.
		if node.creationTimestamp.Add(durationThresholdForDeletion).After(timeNow) {
			continue
		}

		// Skip Machines in NodeGroups that violate the readiness ratio (or have no nodes,
		// which also avoids a division by zero).
		ngStat, ok := ngStatuses[node.group]
		if !ok {
			continue
		}
		if ngStat.nodes <= 0 || float64(ngStat.ready)/float64(ngStat.nodes) < nodeGroupReadinessRatio {
			continue
		}

		candidates = append(candidates, deletionCandidate{name: m.GetName(), nodeCreationTimestamp: node.creationTimestamp})
	}
	return candidates, nil
}

// machinesToDelete sorts candidates oldest-first and returns at most 10% of them
// (always at least one), matching the hook's batch sizing.
func machinesToDelete(candidates []deletionCandidate) []string {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].nodeCreationTimestamp.Before(candidates[j].nodeCreationTimestamp)
	})

	batch := len(candidates) / 10
	if batch == 0 {
		batch = 1
	}

	names := make([]string, 0, batch)
	for _, c := range candidates {
		if len(names) >= batch {
			break
		}
		names = append(names, c.name)
	}
	return names
}

// deleteMachine deletes the MCM Machine; MCM drains the node, deletes the VM and
// recreates a fresh preemptible node.
func (r *Reconciler) deleteMachine(ctx context.Context, name string) error {
	m := &unstructured.Unstructured{}
	m.SetGroupVersionKind(machineGVK)
	m.SetNamespace(nodecommon.MachineNamespace)
	m.SetName(name)

	if err := r.Client.Delete(ctx, m); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
