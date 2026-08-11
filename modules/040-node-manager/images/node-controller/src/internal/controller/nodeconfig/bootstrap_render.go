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
// registered as a Node yet; the caller passes the future node name and adds the
// bootstrap token itself. Otherwise identical to the day-2 render.
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

	// Zero CreationTimestamp makes registration taints render. The labels kubelet
	// will register with must be set too: a NodeExtensionRequest selects by node
	// label, and a bare-name node would miss its extensions until the first
	// day-2 render.
	labels := map[string]string{}
	for key, value := range renderNodeLabels(ng) {
		labels[key] = string(value)
	}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: machineName, Labels: labels}}
	return renderSpec(ng, node, in), nil
}

// resolveKubernetesVersion derives the group's kubelet version from the cluster
// configuration, not the group's status (empty until bashible nodes exist).
// A derivation failure is fatal: the version picks the kubelet system extension.
func resolveKubernetesVersion(ctx context.Context, derived *derived_status.Service, ng *v1.NodeGroup) (string, error) {
	version, err := deriveKubernetesVersion(ctx, derived, ng)
	if err != nil {
		return "", err
	}
	if version != "" {
		return version, nil
	}
	return ng.Status.KubernetesVersion, nil
}

// deriveKubernetesVersion is the cluster's half of the answer above: the version
// derived_status computes from the cluster configuration and the control plane,
// empty when it has none. The group only names the failure it is reported with.
func deriveKubernetesVersion(ctx context.Context, derived *derived_status.Service, ng *v1.NodeGroup) (string, error) {
	// The derived status reports the version even when a later cloud check fails,
	// so the check outcome is ignored here — the error is not.
	computed, _, err := derived.ComputeWithCloudChecks(ctx, ng)
	if err != nil {
		return "", fmt.Errorf("derive the Kubernetes version of %s: %w", ng.Name, err)
	}
	return computed.KubernetesVersion, nil
}
