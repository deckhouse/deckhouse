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
	"strconv"

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

// serviceEndpointSlice is the one EndpointSlice of the Service named after the in-cluster key while
// there is a cache (templates/v2/storage/service.yaml): the Service has no selector then.
//
// The leader, and not every replica, for the same reason the nodes read from it: a follower holds only
// the image set it replicates, while what `d8 mirror push` publishes beside it, such as the security
// databases, is on the leader alone. A pod sent to a follower of an air-gapped cache got 404.
const serviceEndpointSlice = "registry-leader"

// Port names of the Service: the key's and StorageHost's.
const (
	servicePortKey     = "registry"
	servicePortStorage = "storage"
)

// applyServiceEndpoint points the Service at the replica the nodes read from (layout.StorageEndpoint),
// so that pods keep reading wherever the nodes do while the lease moves.
//
// Ready whatever that replica's state, as the nodes are pointed at it whatever its state: a dead leader
// is left behind by both once the lease moves, not sooner. The registry module's hook therefore
// publishes the key for pods once the controller names a replica here, not once that replica answers.
func (r *Reconciler) applyServiceEndpoint(
	ctx context.Context, storage *registryv1alpha1.RegistryStorage, readAddress string,
) (bool, error) {
	// Only the store's own Service without a selector. While the name is the previous implementation's,
	// its slices are a node's pull path until legacy_cleanup.go removes it; the selectorless one follows.
	if ours, err := r.serviceIsSelectorless(ctx); err != nil || !ours {
		return false, err
	}

	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: serviceEndpointSlice, Namespace: Namespace},
	}
	addressType, endpoints := pushEndpoints(readAddress)
	port := servicePort(readAddress)

	existing := &discoveryv1.EndpointSlice{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(slice), existing)
	switch {
	case err == nil && existing.AddressType != addressType:
		if len(endpoints) == 0 {
			addressType = existing.AddressType
		} else if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("replacing %s for another address family: %w", serviceEndpointSlice, err)
		}
	case err != nil && !apierrors.IsNotFound(err):
		return false, fmt.Errorf("reading %s: %w", serviceEndpointSlice, err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, slice, func() error {
		if slice.Labels == nil {
			slice.Labels = map[string]string{}
		}
		slice.Labels[discoveryv1.LabelServiceName] = constant.ServiceName
		slice.Labels[discoveryv1.LabelManagedBy] = pushManagedBy
		slice.Labels["heritage"] = "deckhouse"
		slice.Labels["module"] = "registry"

		slice.AddressType, slice.Endpoints = addressType, endpoints
		slice.Ports = []discoveryv1.EndpointPort{
			{Name: ptr.To(servicePortKey), Port: ptr.To(port), Protocol: ptr.To(corev1.ProtocolTCP)},
			{Name: ptr.To(servicePortStorage), Port: ptr.To(port), Protocol: ptr.To(corev1.ProtocolTCP)},
		}

		// Owned by the cache: without it the Service selects the proxy for pods and the slice goes.
		return controllerutil.SetOwnerReference(storage, slice, r.Client.Scheme())
	})
	if err != nil {
		return false, fmt.Errorf("pointing the Service %s at the storage leader: %w", constant.ServiceName, err)
	}
	return true, r.removeSelectorServiceSlices(ctx)
}

// removeSelectorServiceSlices deletes what the EndpointSlice controller left for the Service from when it
// selected every replica: those slices outlive the selector, and a Service's slices are read together, so
// a pod would still reach a follower through them. Only that controller's, the ones it would recreate
// if the selector came back.
func (r *Reconciler) removeSelectorServiceSlices(ctx context.Context) error {
	slices := &discoveryv1.EndpointSliceList{}
	if err := r.Client.List(ctx, slices, client.InNamespace(Namespace),
		client.MatchingLabels{
			discoveryv1.LabelServiceName: constant.ServiceName,
			discoveryv1.LabelManagedBy:   endpointSliceControllerName,
		}); err != nil {
		return fmt.Errorf("listing the EndpointSlices of %s: %w", constant.ServiceName, err)
	}
	for i := range slices.Items {
		if err := r.Client.Delete(ctx, &slices.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("removing %s, left from when %s had a selector: %w",
				slices.Items[i].Name, constant.ServiceName, err)
		}
	}
	return nil
}

// endpointSliceControllerName is the label value Kubernetes' own EndpointSlice controller manages by.
const endpointSliceControllerName = "endpointslice-controller.k8s.io"

// serviceIsSelectorless reports whether the Service named after the key is the store's, rendered for the
// cache without a selector (templates/v2/storage/service.yaml), and so is this controller's to point.
func (r *Reconciler) serviceIsSelectorless(ctx context.Context) (bool, error) {
	service := &corev1.Service{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: constant.ServiceName, Namespace: Namespace}, service)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading the Service %s: %w", constant.ServiceName, err)
	}
	return service.Labels["app"] == storageServiceLabel && len(service.Spec.Selector) == 0, nil
}

// storageServiceLabel is the app label of the store's Service, as the template sets it.
const storageServiceLabel = "registry-storage"

// servicePort is where the replica serves reads: StoragePort, or LegacyStoragePort for one still on the
// previous pod spec.
func servicePort(address string) int32 {
	if _, port, err := net.SplitHostPort(address); err == nil && port == strconv.Itoa(constant.LegacyStoragePort) {
		return constant.LegacyStoragePort
	}
	return constant.StoragePort
}
