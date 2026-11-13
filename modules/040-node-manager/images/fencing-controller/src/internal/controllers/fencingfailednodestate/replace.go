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

package fencingfailednodestate

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// The groups of the machines. The cluster autoscaler of Deckhouse orders
	// a new machine for a deleted one.
	capiGroup = "cluster.x-k8s.io"
	mcmGroup  = "machine.sapcloud.io"

	// CAPI sets these annotations on the Node of a machine.
	capiMachineAnnotation     = "cluster.x-k8s.io/machine"
	capiClusterNameAnnotation = "cluster.x-k8s.io/cluster-name"

	// staticClusterName is the CAPI cluster of the static machines (CAPS).
	// Nothing replaces a static machine.
	staticClusterName = "static"
)

// The labels of the Nodes that the controller never deletes.
var neverDeletedLabels = []string{
	"node-role.kubernetes.io/control-plane",
	"node-role.kubernetes.io/master",
	"node.deckhouse.io/etcd-arbiter",
}

var (
	instanceList = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha2", Kind: "InstanceList"}
	mhcList      = schema.GroupVersionKind{Group: capiGroup, Version: "v1beta2", Kind: "MachineHealthCheckList"}
	mcmMachine   = schema.GroupVersionKind{Group: mcmGroup, Version: "v1alpha1", Kind: "Machine"}
)

// replacement returns the deletion that gets a new machine for an evicted Node.
// For an MCM machine it deletes the Machine. For a cloud CAPI machine it
// deletes the Node, and the MachineHealthCheck replaces the machine. Other
// Nodes keep only the taint, and the deletion is nil. A master and an etcd
// arbiter are never deleted.
//
// replacement does not delete anything: the controller writes Evicted first.
// It also returns why the controller does not delete the Node of a cloud CAPI
// machine now.
func (r *Reconciler) replacement(ctx context.Context, name string) (func(context.Context) error, string, error) {
	logger := logf.FromContext(ctx)

	var node corev1.Node
	if err := callAPI(ctx, func(ctx context.Context) error {
		return r.reader.Get(ctx, types.NamespacedName{Name: name}, &node)
	}); err != nil {
		return nil, "", fmt.Errorf("get node %q: %w", name, err)
	}

	for _, label := range neverDeletedLabels {
		if _, ok := node.Labels[label]; ok {
			logger.Info("node is never deleted, it keeps only the out-of-service taint", "node", name, "label", label)

			return nil, "", nil
		}
	}

	group, machine, err := r.machineOf(ctx, &node)
	if err != nil {
		return nil, "", err
	}

	cluster := node.Annotations[capiClusterNameAnnotation]

	switch {
	case group == mcmGroup:
		return func(ctx context.Context) error { return r.deleteMCMMachine(ctx, name, machine) }, "", nil
	case group == capiGroup && cluster != "" && cluster != staticClusterName:
		allowed, refused, err := r.healthChecksAllow(ctx, &node, cluster)
		if err != nil || refused != "" {
			return nil, refused, err
		}

		return func(ctx context.Context) error { return r.deleteCAPINode(ctx, &node, cluster, allowed) }, "", nil
	}

	logger.Info("node is not replaced, it keeps only the out-of-service taint",
		"node", name, "machine_group", group, "capi_cluster", cluster)

	return nil, "", nil
}

// replace sends again the deletion that gets a new machine for the Node of an
// Evicted incident. The first deletion can fail after the write of Evicted. A
// deletion that the API server already has changes nothing. When the
// MachineHealthChecks do not replace the machine now, the controller examines
// the Node again after recheckAfter.
func (r *Reconciler) replace(ctx context.Context, name string) (ctrl.Result, error) {
	deletion, refused, err := r.replacement(ctx, name)

	switch {
	case apierrors.IsNotFound(err):
		// The Node is gone, so nothing is left to delete.
		return ctrl.Result{}, nil
	case err != nil:
		return ctrl.Result{}, err
	case refused != "":
		logf.FromContext(ctx).Info("node of the evicted incident is not deleted", "node", name, "reason", refused)

		return ctrl.Result{RequeueAfter: recheckAfter}, nil
	case deletion == nil:
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, deletion(ctx)
}

// machineOf returns the group and the namespaced name of the machine of the
// Node. The Instance of the Node gives the machine in spec.machineRef. A Node
// without such an Instance can have the machine annotation of CAPI. The group
// is empty when the Node has no machine.
func (r *Reconciler) machineOf(ctx context.Context, node *corev1.Node) (string, types.NamespacedName, error) {
	instances := &unstructured.UnstructuredList{}
	instances.SetGroupVersionKind(instanceList)

	if err := callAPI(ctx, func(ctx context.Context) error {
		return r.reader.List(ctx, instances)
	}); err != nil {
		return "", types.NamespacedName{}, fmt.Errorf("list instances: %w", err)
	}

	for i := range instances.Items {
		item := instances.Items[i].Object
		if nodeName, _, _ := unstructured.NestedString(item, "spec", "nodeRef", "name"); nodeName != node.Name {
			continue
		}

		ref, ok, _ := unstructured.NestedStringMap(item, "spec", "machineRef")
		if !ok || ref["name"] == "" {
			break
		}

		machine := types.NamespacedName{Namespace: ref["namespace"], Name: ref["name"]}
		if machine.Namespace == "" {
			machine.Namespace = r.namespace
		}

		group, _, _ := strings.Cut(ref["apiVersion"], "/")

		return group, machine, nil
	}

	if name := node.Annotations[capiMachineAnnotation]; name != "" {
		return capiGroup, types.NamespacedName{Namespace: r.namespace, Name: name}, nil
	}

	return "", types.NamespacedName{}, nil
}

// deleteMCMMachine deletes the MCM Machine. MCM orders a new machine.
func (r *Reconciler) deleteMCMMachine(ctx context.Context, node string, name types.NamespacedName) error {
	machine := &unstructured.Unstructured{}
	machine.SetGroupVersionKind(mcmMachine)
	machine.SetNamespace(name.Namespace)
	machine.SetName(name.Name)

	if err := r.client.Delete(ctx, machine); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete machine %s of node %q: %w", name, node, err)
	}

	logf.FromContext(ctx).Info("machine of the evicted node is deleted", "node", node, "machine", name.String())

	return nil
}

// healthChecksAllow tells if the deletion of the Node of a cloud CAPI machine
// gets a new machine: at most half of the machines of the CAPI cluster are
// unhealthy. Then the MachineHealthCheck replaces the machine, because its Node
// is gone. When more machines are unhealthy, the MachineHealthCheck does not
// replace machines. Then healthChecksAllow returns the reason, and the
// controller keeps the Node.
//
// healthChecksAllow returns the MachineHealthChecks that allow the deletion,
// as "name: currentHealthy of expectedMachines".
func (r *Reconciler) healthChecksAllow(ctx context.Context, node *corev1.Node, cluster string) ([]string, string, error) {
	checks := &unstructured.UnstructuredList{}
	checks.SetGroupVersionKind(mhcList)

	if err := callAPI(ctx, func(ctx context.Context) error {
		return r.reader.List(ctx, checks, client.InNamespace(r.namespace))
	}); err != nil {
		return nil, "", fmt.Errorf("list machinehealthchecks: %w", err)
	}

	// MachineHealthCheck counts the machine as unhealthy only after its Node
	// is not Ready. The eviction can come before that. Then the controller
	// counts the machine as unhealthy itself.
	notCounted := int64(0)
	if nodeReady(node) {
		notCounted = 1
	}

	var allowed []string

	for i := range checks.Items {
		check := &checks.Items[i]
		if name, _, _ := unstructured.NestedString(check.Object, "spec", "clusterName"); name != cluster {
			continue
		}

		expected, hasExpected, _ := unstructured.NestedInt64(check.Object, "status", "expectedMachines")
		healthy, hasHealthy, _ := unstructured.NestedInt64(check.Object, "status", "currentHealthy")

		if !hasExpected || !hasHealthy || expected == 0 {
			return nil, fmt.Sprintf("MachineHealthCheck %s of the CAPI cluster %q does not report its machines yet, so the node is not deleted.",
				check.GetName(), cluster), nil
		}

		if unhealthy := min(expected, expected-healthy+notCounted); 2*unhealthy > expected {
			return nil, fmt.Sprintf("%d of %d machines of the CAPI cluster %q are unhealthy, more than 50%%, so MachineHealthCheck %s does not replace a machine and the node is not deleted.",
				unhealthy, expected, cluster, check.GetName()), nil
		}

		allowed = append(allowed, fmt.Sprintf("%s: %d of %d", check.GetName(), healthy, expected))
	}

	if len(allowed) == 0 {
		return nil, fmt.Sprintf("The CAPI cluster %q has no MachineHealthCheck that replaces a machine, so the node is not deleted.", cluster), nil
	}

	return allowed, "", nil
}

// deleteCAPINode deletes the Node of a cloud CAPI machine. The
// MachineHealthChecks in allowed replace the machine.
func (r *Reconciler) deleteCAPINode(ctx context.Context, node *corev1.Node, cluster string, allowed []string) error {
	// The UID keeps a new Node with the same name.
	uid := node.UID
	if err := r.client.Delete(ctx, node, client.Preconditions{UID: &uid}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete node %q: %w", node.Name, err)
	}

	logf.FromContext(ctx).Info("node of the cloud machine is deleted, MachineHealthCheck replaces the machine",
		"node", node.Name, "capi_cluster", cluster, "node_ready", nodeReady(node),
		"machine_health_checks_current_healthy_of_expected", allowed)

	return nil
}

// nodeReady tells if the Node has the condition Ready with the status True.
func nodeReady(node *corev1.Node) bool {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == corev1.NodeReady {
			return node.Status.Conditions[i].Status == corev1.ConditionTrue
		}
	}

	return false
}
