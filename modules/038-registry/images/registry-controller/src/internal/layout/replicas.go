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

package layout

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// ControlPlaneLabel marks the nodes a storage replica runs on: the StatefulSet is placed with
// helm_lib_node_selector "master", which selects by it.
const ControlPlaneLabel = "node-role.kubernetes.io/control-plane"

// StorageNodes are the nodes a storage replica is expected on, sorted as listed.
func StorageNodes(nodes []corev1.Node) []string {
	var names []string
	for i := range nodes {
		if _, isMaster := nodes[i].Labels[ControlPlaneLabel]; isMaster {
			names = append(names, nodes[i].Name)
		}
	}
	return names
}

// ListStorageNodes lists the nodes a storage replica is expected on, and only those: a cluster has
// hundreds of nodes and three masters.
func ListStorageNodes(ctx context.Context, reader client.Reader) ([]string, error) {
	nodes := &corev1.NodeList{}
	if err := reader.List(ctx, nodes, client.HasLabels{ControlPlaneLabel}); err != nil {
		return nil, fmt.Errorf("listing the storage nodes: %w", err)
	}
	return StorageNodes(nodes.Items), nil
}

// StorageNodesChanged lets through a storage node coming or going: a master created or deleted, or a
// node gaining or losing the label that makes it one. Kubelet's status updates on every node are not
// news to anything that only asks which replicas are expected.
func StorageNodesChanged() predicate.Predicate {
	isStorage := func(object client.Object) bool {
		_, isMaster := object.GetLabels()[ControlPlaneLabel]
		return isMaster
	}
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return isStorage(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return isStorage(e.Object) },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isStorage(e.ObjectOld) != isStorage(e.ObjectNew)
		},
	}
}

// CurrentReplicas are the entries of the replicas that are still expected: on a storage node.
//
// An entry outlives its pod — nothing removes it when a master goes — so a status read as the list of
// replicas names one that will never answer. Anything that waits for every replica, or asks every one
// to do something, has to read this instead.
func CurrentReplicas(
	replicas []registryv1alpha1.StorageReplicaStatus, storageNodes []string,
) []registryv1alpha1.StorageReplicaStatus {
	expected := make(map[string]bool, len(storageNodes))
	for _, node := range storageNodes {
		expected[node] = true
	}
	var current []registryv1alpha1.StorageReplicaStatus
	for _, replica := range replicas {
		if expected[replica.Node] {
			current = append(current, replica)
		}
	}
	return current
}
