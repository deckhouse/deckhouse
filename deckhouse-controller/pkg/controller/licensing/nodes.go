// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package licensing

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Taints that reserve a node for the platform. control-plane and its legacy
// spelling reserve it whatever the value; dedicated.deckhouse.io only for the
// two values below.
const (
	taintControlPlane = "node-role.kubernetes.io/control-plane"
	taintMaster       = "node-role.kubernetes.io/master"
	taintDedicated    = "dedicated.deckhouse.io"
)

// dedicatedFreeValues are the only values of dedicated.deckhouse.io that make a
// node free. Everything else is licensed, and frontend in particular: a frontend
// node carries the ingress traffic of the customer, which is exactly the load a
// licence is about.
var dedicatedFreeValues = map[string]bool{"system": true, "monitoring": true}

// systemNamespacePrefixes are the namespaces whose pods are platform, not user,
// workload.
var systemNamespacePrefixes = []string{"d8-", "kube-"}

// nodeObservation is one node as the licensing sees it.
type nodeObservation struct {
	Name string
	VCPU int64
	// Free is true when the node carries no licence cost; Reason then says why.
	Free   bool
	Reason string
}

// classify applies the free node rule of specification 7.1. A node is free when
// it is reserved for the platform by a taint and actually runs no user workload;
// everything else is licensable, NotReady nodes included, because it is capacity
// that is licensed and the state is temporary.
//
// A node whose pods cannot be listed aborts the whole observation: leaving it
// out would silently understate consumption, and no later reconcile would
// notice.
func classify(nodes []corev1.Node, hasUserPods func(nodeName string) (bool, error)) ([]nodeObservation, error) {
	out := make([]nodeObservation, 0, len(nodes))
	for _, node := range nodes {
		observed := nodeObservation{Name: node.Name, VCPU: wholeCores(node)}

		if reason, reserved := reservation(node); reserved {
			used, err := hasUserPods(node.Name)
			if err != nil {
				return nil, err
			}
			if !used {
				observed.Free = true
				observed.Reason = reason + ", no user pods"
			}
		}

		out = append(out, observed)
	}
	// The API server lists nodes in no guaranteed order, and this slice decides
	// how the published one is built.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// reservation reports whether a taint reserves the node for the platform, and
// names the taint for the status.
func reservation(node corev1.Node) (string, bool) {
	for _, taint := range node.Spec.Taints {
		switch {
		case taint.Key == taintControlPlane:
			return "control-plane", true
		case taint.Key == taintMaster:
			return "control-plane", true
		case taint.Key == taintDedicated && dedicatedFreeValues[taint.Value]:
			return taintDedicated + "=" + taint.Value, true
		}
	}
	return "", false
}

// licensable returns the nodes the metrics and the allocation are computed over.
func licensable(observed []nodeObservation) []licensing.Node {
	out := make([]licensing.Node, 0, len(observed))
	for _, node := range observed {
		if node.Free {
			continue
		}
		out = append(out, licensing.Node{Name: node.Name, VCPU: node.VCPU})
	}
	return out
}

func freeNodes(observed []nodeObservation) int64 {
	var count int64
	for _, node := range observed {
		if node.Free {
			count++
		}
	}
	return count
}

// wholeCores rounds the capacity up: a node advertising 3500m is licensed as
// four vCPU, never as three.
func wholeCores(node corev1.Node) int64 {
	milli := node.Status.Capacity.Cpu().MilliValue()
	if milli <= 0 {
		return 0
	}
	return (milli + 999) / 1000
}

// observeNodes takes one reading of the cluster node set.
func (r *reconciler) observeNodes(ctx context.Context) ([]nodeObservation, error) {
	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// ponytail: one pod list per reserved node. That is a handful of calls on
	// any real cluster; if a fleet ever shows up with hundreds of platform
	// nodes, replace it with a single cluster wide pod list.
	return classify(nodes.Items, func(name string) (bool, error) {
		return r.hasUserPods(ctx, name)
	})
}

// hasUserPods reports whether a reserved node runs at least one pod that is
// neither platform workload nor a per-node agent. The list goes through the
// uncached reader on purpose: the manager caches only a narrow slice of pods.
func (r *reconciler) hasUserPods(ctx context.Context, nodeName string) (bool, error) {
	var pods corev1.PodList
	err := r.apiReader.List(ctx, &pods, client.MatchingFields{"spec.nodeName": nodeName})
	if err != nil {
		// No reading at all beats a wrong one: the reconcile fails, the
		// controller backs off and the observation is taken when the API
		// answers again.
		r.logger.Warn("list pods of a reserved node", slog.String("node", nodeName), log.Err(err))
		return false, fmt.Errorf("list pods of node %s: %w", nodeName, err)
	}

	for _, pod := range pods.Items {
		if isUserPod(pod) {
			return true, nil
		}
	}
	return false, nil
}

func isUserPod(pod corev1.Pod) bool {
	// A finished Job leaves its pod object behind for hours. It is not running
	// workload, so it must not make a reserved node licensable.
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	for _, prefix := range systemNamespacePrefixes {
		if strings.HasPrefix(pod.Namespace, prefix) {
			return false
		}
	}
	// A DaemonSet lands on every node it tolerates, so its presence says nothing
	// about the node being opened up for user workload.
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return false
		}
	}
	return true
}
