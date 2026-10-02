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
	"net"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const (
	// PushService is the Service the publication endpoint's ingress sends a push to. Rendered by
	// Helm without a selector: which replica it reaches is decided here.
	PushService = "registry-push"

	// pushEndpointSlice is the one EndpointSlice of PushService, and this controller its only writer.
	pushEndpointSlice = "registry-push-leader"

	// pushPortName names the Service port the slice serves: for a Service without a selector, the
	// port of an endpoint is matched to the Service's by name.
	pushPortName = "registry-push"

	// pushManagedBy tells the EndpointSlice controller that this slice is not its to reconcile.
	pushManagedBy = "registry-controller.deckhouse.io"
)

// applyPushEndpoint points the publication endpoint at the storage leader, and at nothing else.
//
// A push has to land where the cluster reads from. Agents pull from the leader first, and a follower
// answering for an image it holds is never asked — a 404 from the leader does not fall through to the
// mirrors, deliberately. Replication runs from the leader to the followers and never back. So a push
// that the ingress sent to a follower, which a Service over every replica did whenever the client's
// address hashed that way, landed in a store nobody reads and nothing copies from: an air-gapped
// cluster never saw the release pushed into it. Through the leader, it is on the replica the cluster
// pulls from, and the followers replicate it from there — the same way everything else reaches them.
//
// The leader is the one the election lease names, the same answer the agents are given, read from the
// same place, so the two cannot disagree about which replica that is. With no leader the slice is
// empty and a push is refused at the ingress, which is the honest answer: there is no replica whose
// store the cluster would read it from.
//
// Owned by the RegistryStorage, which exists exactly while the cache does, so turning the cache off
// takes the slice with it.
func (r *Reconciler) applyPushEndpoint(
	ctx context.Context, storage *registryv1alpha1.RegistryStorage, leaderAddress string,
) error {
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: pushEndpointSlice, Namespace: Namespace},
	}
	addressType, endpoints := pushEndpoints(leaderAddress)

	// The address type of a slice cannot change. A leader on the other family replaces the slice;
	// an empty slice keeps the family it has, since it names no address to disagree with.
	existing := &discoveryv1.EndpointSlice{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(slice), existing)
	switch {
	case err == nil && existing.AddressType != addressType:
		if len(endpoints) == 0 {
			addressType = existing.AddressType
		} else if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("replacing %s for another address family: %w", pushEndpointSlice, err)
		}
	case err != nil && !apierrors.IsNotFound(err):
		return fmt.Errorf("reading %s: %w", pushEndpointSlice, err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, slice, func() error {
		if slice.Labels == nil {
			slice.Labels = map[string]string{}
		}
		slice.Labels[discoveryv1.LabelServiceName] = PushService
		slice.Labels[discoveryv1.LabelManagedBy] = pushManagedBy
		slice.Labels["heritage"] = "deckhouse"
		slice.Labels["module"] = "registry"

		slice.AddressType, slice.Endpoints = addressType, endpoints
		slice.Ports = []discoveryv1.EndpointPort{{
			Name:     ptr.To(pushPortName),
			Port:     ptr.To(int32(constant.WritePort)),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}}

		return controllerutil.SetOwnerReference(storage, slice, r.Client.Scheme())
	})
	if err != nil {
		return fmt.Errorf("pointing %s at the storage leader: %w", PushService, err)
	}
	return r.removeOtherPushSlices(ctx)
}

// removeOtherPushSlices deletes every EndpointSlice of PushService but this controller's.
//
// The Service had a selector until the publication endpoint was pointed at the leader, and the
// EndpointSlice controller does not take back the slices it wrote once the selector is gone: they stay,
// naming every replica, and the ingress went on spreading a push across all of them — whose upload
// then landed on one master and its next chunk on another, BLOB_UPLOAD_UNKNOWN. On a cluster that ran
// the selector it is every slice but ours; on any other, nothing.
func (r *Reconciler) removeOtherPushSlices(ctx context.Context) error {
	slices := &discoveryv1.EndpointSliceList{}
	if err := r.Client.List(ctx, slices, client.InNamespace(Namespace),
		client.MatchingLabels{discoveryv1.LabelServiceName: PushService}); err != nil {
		return fmt.Errorf("listing the EndpointSlices of %s: %w", PushService, err)
	}
	for i := range slices.Items {
		slice := &slices.Items[i]
		if slice.Name == pushEndpointSlice {
			continue
		}
		if err := r.Client.Delete(ctx, slice); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("removing %s, left from when %s had a selector: %w", slice.Name, PushService, err)
		}
	}
	return nil
}

// pushEndpoints is the leader's write listener as an endpoint: the host of the address it serves on,
// which for a host-networked replica is its node's.
func pushEndpoints(leaderAddress string) (discoveryv1.AddressType, []discoveryv1.Endpoint) {
	host := leaderAddress
	if h, _, err := net.SplitHostPort(leaderAddress); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// No leader, or an address that is not one: nothing may receive a push.
		return discoveryv1.AddressTypeIPv4, []discoveryv1.Endpoint{}
	}

	addressType := discoveryv1.AddressTypeIPv4
	if ip.To4() == nil {
		addressType = discoveryv1.AddressTypeIPv6
	}
	return addressType, []discoveryv1.Endpoint{{
		Addresses:  []string{ip.String()},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
	}}
}
