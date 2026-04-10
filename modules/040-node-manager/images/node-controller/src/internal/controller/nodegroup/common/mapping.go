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

package common

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NodeToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}
	ngName, exists := node.Labels[NodeGroupLabel]
	if !exists {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}

func MachineToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	ngName := labels[NodeGroupLabel]
	if ngName == "" {
		ngName = labels["node-group"]
	}
	if ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}

func MachineDeploymentToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ngName := obj.GetLabels()["node-group"]
	if ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}
