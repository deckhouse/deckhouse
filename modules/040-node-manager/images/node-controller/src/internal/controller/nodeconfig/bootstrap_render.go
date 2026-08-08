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
	"fmt"

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
	version, err := resolveKubernetesVersion(ctx, derived, ng)
	if err != nil {
		return internalv1alpha1.NodeSpec{}, err
	}

	sources := &sourceReader{Client: cl, Reader: reader}
	in, err := sources.readClusterInputs(ctx, version)
	if err != nil {
		return internalv1alpha1.NodeSpec{}, err
	}

	// A node that has not joined yet: the zero CreationTimestamp makes the
	// registration taints render, and the machine name is what kubelet will
	// register the node under. It carries the labels kubelet is about to register
	// it with, because a NodeExtensionRequest can select by node label — a
	// machine built from a bare name matched none of them, so the node booted
	// without an extension it was selected for and picked it up on its first
	// day-2 render instead: a spec change, and possibly an interruption, minutes
	// after joining.
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   machineName,
		Labels: registrationLabels(ng),
	}}
	return renderSpec(ng, node, in), nil
}

// resolveKubernetesVersion is the version a group's kubelet must match. It is
// derived from the cluster configuration rather than read from the group's
// status, which is only filled once the group has bashible-managed nodes.
//
// A failure to derive it is fatal to the render rather than a fall back on the
// status: the version picks the kubelet system extension, so falling back would
// hand the group a different kubelet — and an immutable group has no
// bashible-managed nodes, so its status version is usually empty anyway.
func resolveKubernetesVersion(ctx context.Context, derived *derived_status.Service, ng *v1.NodeGroup) (string, error) {
	// The derived status reports the version even when a later cloud check fails,
	// so the check outcome is ignored here — the error is not.
	computed, _, err := derived.ComputeWithCloudChecks(ctx, ng)
	if err != nil {
		return "", fmt.Errorf("derive the Kubernetes version of %s: %w", ng.Name, err)
	}
	if computed.KubernetesVersion != "" {
		return computed.KubernetesVersion, nil
	}
	return ng.Status.KubernetesVersion, nil
}
