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

package nodeconfig

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
)

// RenderBootstrapSpec renders the NodeConfig spec for a machine that has not
// registered as a Node yet, for the bootstrap provider. There is no Node object
// at bootstrap time, so the caller passes the name the node will register
// under; the spec is otherwise the same one the day-2 controller renders, from
// the same live cluster state. The bootstrap token kubelet needs on first
// contact is not filled in here — the day-2 path carries none, so the caller
// adds it.
func RenderBootstrapSpec(ctx context.Context, cl client.Client, reader client.Reader, ng *v1.NodeGroup, machineName string) (internalv1alpha1.NodeSpec, error) {
	derived := &derived_status.Service{Client: cl, Reader: reader}
	version := resolveKubernetesVersion(ctx, derived, ng)

	sources := &sourceReader{Client: cl, Reader: reader}
	in, err := sources.readClusterInputs(ctx, version)
	if err != nil {
		return internalv1alpha1.NodeSpec{}, err
	}

	// A node that has not joined yet: the zero CreationTimestamp makes the
	// registration taints render, and the machine name is what kubelet will
	// register the node under.
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: machineName}}
	return renderSpec(ng, node, in), nil
}

// resolveKubernetesVersion is the version a group's kubelet must match. It is
// derived from the cluster configuration rather than read from the group's
// status, which is only filled once the group has bashible-managed nodes.
func resolveKubernetesVersion(ctx context.Context, derived *derived_status.Service, ng *v1.NodeGroup) string {
	// The derived status reports the version even when a later cloud check fails,
	// so the check outcome is ignored here.
	computed, _, _ := derived.ComputeWithCloudChecks(ctx, ng)
	if computed.KubernetesVersion != "" {
		return computed.KubernetesVersion
	}
	return ng.Status.KubernetesVersion
}
