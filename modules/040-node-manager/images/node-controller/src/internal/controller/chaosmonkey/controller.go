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

// Package chaosmonkey periodically kills one random node of every NodeGroup whose
// spec.chaos.mode is DrainAndDelete, by deleting the node's MCM Machine.
//
// This replaces the shell-operator hook hooks/chaos_monkey.go. The hook was
// schedule-driven (crontab "* * * * *") with only passive Kubernetes bindings, so
// this controller reproduces it with a one-minute ticker rather than a reactive
// watch — NodeGroup/Node/Machine events must NOT trigger it, or the per-minute
// probability gate would fire more often and make chaos more aggressive.
//
// Scope: MCM only (machine.sapcloud.io/v1alpha1), matching the hook 1:1. CAPI-backed
// NodeGroups (cluster.x-k8s.io) are not handled yet — see the migration TODO.
package chaosmonkey

import (
	"context"
	"math/rand"
	"os"
	"strconv"
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
	victimKey          = "node.deckhouse.io/chaos-monkey-victim"
	nodeGroupLabel     = "node.deckhouse.io/group"
	machineNodeLabel   = "node"
	defaultChaosPeriod = "6h"
	tickInterval       = time.Minute
)

var machineGVK = schema.GroupVersionKind{
	Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "Machine",
}

func newMachineList() *unstructured.UnstructuredList {
	l := &unstructured.UnstructuredList{}
	l.SetGroupVersionKind(machineGVK.GroupVersion().WithKind("MachineList"))
	return l
}

func init() {
	register.RegisterController("chaos-monkey", &deckhousev1.NodeGroup{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Drop all primary (NodeGroup) events: the hook was schedule-only, so reactive
	// triggers would run the probability gate more often than once a minute.
	w.WithEventFilter(predicate.NewPredicateFuncs(func(client.Object) bool { return false }))

	// One-minute ticker reproducing the hook's crontab "* * * * *". Raw sources are
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
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: "chaos-monkey"}})
				}
			}
		}()
		return nil
	}))
}

type chaosNodeGroup struct {
	name    string
	mode    deckhousev1.ChaosMode
	period  string
	isReady bool
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	randomizer := rand.New(rand.NewSource(randomSeed()))

	machinesByNode, hasVictim, err := r.collectMachines(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Global gate: if any Machine is already a victim, an earlier deletion is still
	// in progress — do nothing this tick (parity with the hook's early return).
	if hasVictim {
		logger.Info("a chaos-monkey victim already exists, skipping this tick")
		return ctrl.Result{}, nil
	}

	nodeGroups, err := r.chaosNodeGroups(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	nodesByGroup, err := r.nodesByGroup(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, ng := range nodeGroups {
		if ng.mode != deckhousev1.ChaosModeDrainAndDelete {
			continue
		}

		chaosPeriod, err := time.ParseDuration(ng.period)
		if err != nil {
			logger.Info("chaos period for NodeGroup is invalid", "period", ng.period, "nodeGroup", ng.name)
			continue
		}

		// periodMinutes is the hook's rand modulus. Guard against zero (sub-minute
		// periods) to avoid a division panic that would crash the whole controller
		// binary — unlike the isolated hook, which would only fail its own run.
		periodMinutes := chaosPeriod.Milliseconds() / 1000 / 60
		if periodMinutes <= 0 {
			logger.Info("chaos period for NodeGroup is shorter than a minute, skipping", "period", ng.period, "nodeGroup", ng.name)
			continue
		}

		if randomizer.Uint32()%uint32(periodMinutes) != 0 {
			continue
		}

		groupNodes := nodesByGroup[ng.name]
		if len(groupNodes) == 0 {
			continue
		}

		victimNode := groupNodes[randomizer.Intn(len(groupNodes))]
		machineName, ok := machinesByNode[victimNode]
		if !ok {
			continue
		}

		if err := r.markAndDeleteMachine(ctx, machineName); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("chaos monkey deleted a machine", "machine", machineName, "node", victimNode, "nodeGroup", ng.name)
	}

	return ctrl.Result{}, nil
}

// collectMachines maps node name to the MCM Machine that runs it and reports whether
// any Machine is already flagged as a chaos-monkey victim (the gate is keyed on the
// label, matching the hook, even though the flag is written as an annotation).
func (r *Reconciler) collectMachines(ctx context.Context) (map[string]string, bool, error) {
	machines := newMachineList()
	if err := r.Client.List(ctx, machines, client.InNamespace(nodecommon.MachineNamespace)); err != nil {
		return nil, false, err
	}

	byNode := make(map[string]string, len(machines.Items))
	hasVictim := false
	for i := range machines.Items {
		m := &machines.Items[i]
		labels := m.GetLabels()
		if _, ok := labels[victimKey]; ok {
			hasVictim = true
		}
		if node := labels[machineNodeLabel]; node != "" {
			byNode[node] = m.GetName()
		}
	}
	return byNode, hasVictim, nil
}

// chaosNodeGroups returns NodeGroups that have chaos enabled and are ready for it.
func (r *Reconciler) chaosNodeGroups(ctx context.Context) ([]chaosNodeGroup, error) {
	ngList := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return nil, err
	}

	result := make([]chaosNodeGroup, 0, len(ngList.Items))
	for i := range ngList.Items {
		ng := &ngList.Items[i]
		if ng.Spec.Chaos == nil || ng.Spec.Chaos.Mode == "" || !isReadyForChaos(ng) {
			continue
		}
		period := ng.Spec.Chaos.Period
		if period == "" {
			period = defaultChaosPeriod
		}
		result = append(result, chaosNodeGroup{
			name:    ng.Name,
			mode:    ng.Spec.Chaos.Mode,
			period:  period,
			isReady: true,
		})
	}
	return result, nil
}

// isReadyForChaos mirrors the hook: a cloud group is ready when its desired count is
// above one and fully satisfied; any other group uses the node counts instead.
func isReadyForChaos(ng *deckhousev1.NodeGroup) bool {
	if ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral {
		return ng.Status.Desired > 1 && ng.Status.Desired == ng.Status.Ready
	}
	return ng.Status.Nodes > 1 && ng.Status.Nodes == ng.Status.Ready
}

// nodesByGroup lists nodes labelled with their NodeGroup and groups them by it.
func (r *Reconciler) nodesByGroup(ctx context.Context) (map[string][]string, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return nil, err
	}

	byGroup := make(map[string][]string)
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		group := node.Labels[nodeGroupLabel]
		if group == "" {
			continue
		}
		byGroup[group] = append(byGroup[group], node.Name)
	}
	return byGroup, nil
}

// markAndDeleteMachine flags the Machine as a victim (annotation, parity with the
// hook) and then deletes it; MCM drains the node, deletes the VM and recreates it.
func (r *Reconciler) markAndDeleteMachine(ctx context.Context, name string) error {
	machine := &unstructured.Unstructured{}
	machine.SetGroupVersionKind(machineGVK)
	machine.SetNamespace(nodecommon.MachineNamespace)
	machine.SetName(name)

	patch := []byte(`{"metadata":{"annotations":{"` + victimKey + `":""}}}`)
	if err := r.Client.Patch(ctx, machine, client.RawPatch(types.MergePatchType, patch)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := r.Client.Delete(ctx, machine); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// randomSeed uses the wall clock, overridable by D8_TEST_RANDOM_SEED for tests
// (same knob the hook exposed).
func randomSeed() int64 {
	if s := os.Getenv("D8_TEST_RANDOM_SEED"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
	}
	return time.Now().UnixNano()
}
