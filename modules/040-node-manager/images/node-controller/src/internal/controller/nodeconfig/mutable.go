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
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// mutableInputs is what a Mutable node's document is rendered from. Far less
// than clusterInputs: no OS image, no system extensions, no kubelet settings —
// those exist only for an Engine node, and a CSE build does not even ship them.
type mutableInputs struct {
	APIServerEndpoints         []string
	RegistryPackagesProxyToken string
	// NodeStaticPodRequests in contest order, with the ones this controller refused.
	NodeStaticPodRequests         []*deckhousev1alpha1.NodeStaticPodRequest
	NodeStaticPodRequestsRejected map[string]nsprRefusal
}

// readMutableInputs reads what a bashible node's document needs, and nothing
// else: readClusterInputs would fail on the release ConfigMap a CSE build does
// not ship, leaving every such node without a document at all.
func (s *sourceReader) readMutableInputs(ctx context.Context) (mutableInputs, error) {
	var in mutableInputs
	endpoints, err := s.readAPIServerEndpoints(ctx)
	if err != nil {
		return in, err
	}
	if len(endpoints) == 0 {
		return in, errors.New("no API server endpoints discovered")
	}
	in.APIServerEndpoints = endpoints

	in.RegistryPackagesProxyToken, err = s.readPackagesProxyToken(ctx)
	if err != nil {
		return in, err
	}

	nsprs, err := s.readNodeStaticPodRequests(ctx)
	if err != nil {
		return in, err
	}
	in.NodeStaticPodRequests = orderedNSPRs(nsprs)
	in.NodeStaticPodRequestsRejected = rejectedNSPRs(in.NodeStaticPodRequests)
	return in, nil
}

// newMutableNodeConfig renders the document of a node bashible configures.
// spec.images is left out: the platform preload list is pause and registry-agent,
// both of which bashible already puts on such a node.
func newMutableNodeConfig(ng *v1.NodeGroup, node *corev1.Node, in mutableInputs) *internalv1alpha1.NodeConfig {
	return &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
			Labels: map[string]string{
				nodecommon.NodeGroupLabel: ng.Name,
				managedByLabel:            managedByValue,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Node",
				Name:       node.Name,
				UID:        node.UID,
			}},
		},
		Spec: internalv1alpha1.NodeSpec{
			SystemType:                          internalv1alpha1.SystemTypeMutable,
			NodeName:                            node.Name,
			APIServerEndpoints:                  in.APIServerEndpoints,
			RegistryPackagesProxyAccessTokenB64: in.RegistryPackagesProxyToken,
			StaticPods:                          nodeStaticPods(in.NodeStaticPodRequests, in.NodeStaticPodRequestsRejected, ng.Name),
		},
	}
}

// reconcileMutableNode keeps the document of a node bashible configures. It waits
// for no rollout slot and asks for no disruption approval: a static pod or an
// image interrupts nothing, and bashible hands its steps to every node at once too.
func (r *Reconciler) reconcileMutableNode(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, logger logr.Logger, p *pass) error {
	inputs, err := r.mutableInputs(ctx, p)
	if err != nil {
		return err
	}
	desired := newMutableNodeConfig(ng, node, inputs)

	existing := &internalv1alpha1.NodeConfig{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	if apierrors.IsNotFound(err) {
		// Always created here: a Mutable node has no file to register one from.
		if err := r.Client.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create NodeConfig %s: %w", desired.Name, err)
		}
		logger.Info("NodeConfig created", "node", desired.Name, "systemType", desired.Spec.SystemType)
		return nil
	}
	if err != nil {
		return fmt.Errorf("get NodeConfig %s: %w", desired.Name, err)
	}
	if removed, err := r.removeOnSystemTypeChange(ctx, existing, desired, logger); removed || err != nil {
		return err
	}
	if upToDate(existing, desired) {
		return nil
	}

	patch := client.MergeFromWithOptions(existing.DeepCopy(), client.MergeFromWithOptimisticLock{})
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.OwnerReferences = desired.OwnerReferences
	if err := r.Client.Patch(ctx, existing, patch); err != nil {
		if apierrors.IsConflict(err) {
			logger.V(1).Info("NodeConfig changed while it was being rendered; leaving it to the next pass", "node", desired.Name)
			return nil
		}
		return fmt.Errorf("patch NodeConfig %s: %w", desired.Name, err)
	}
	logger.Info("NodeConfig updated", "node", desired.Name)
	return nil
}

// removeOnSystemTypeChange deletes an object rendered for the other kind of node,
// since spec.systemType is immutable and its patch would fail for ever. One this
// controller does not manage is left alone: a delete, unlike a patch, is final.
func (r *Reconciler) removeOnSystemTypeChange(ctx context.Context, existing, desired *internalv1alpha1.NodeConfig, logger logr.Logger) (bool, error) {
	from, to := systemTypeOf(existing), systemTypeOf(desired)
	if from == to {
		return false, nil
	}
	if existing.Labels[managedByLabel] != managedByValue {
		logger.Info("NodeConfig kept: its system type does not match the node's group, but this controller does not manage it",
			"node", existing.Name, "from", from, "to", to)
		return true, nil
	}
	if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
		return true, fmt.Errorf("delete NodeConfig %s: %w", existing.Name, err)
	}
	logger.Info("NodeConfig removed: the node changed system type", "node", existing.Name, "from", from, "to", to)
	return true, nil
}

// systemTypeOf reads an absent value as Immutable, the way the CRD defaults it:
// every document written before the field existed is an Engine node's.
func systemTypeOf(nc *internalv1alpha1.NodeConfig) internalv1alpha1.SystemType {
	if nc.Spec.SystemType == "" {
		return internalv1alpha1.SystemTypeImmutable
	}
	return nc.Spec.SystemType
}
