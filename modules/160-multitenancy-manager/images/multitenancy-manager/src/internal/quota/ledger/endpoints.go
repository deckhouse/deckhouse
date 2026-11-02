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

package ledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Namespace is the namespace of the module.
	Namespace = "d8-multitenancy-manager"
	// ServiceName is the Service without a selector that the webhook calls; its EndpointSlice is
	// written by the ledger itself, so the route to the leader does not wait for kube-controller-manager.
	ServiceName = "multitenancy-manager-quota-ledger"
	// LeaseName is the lease of the ledger.
	LeaseName = "multitenancy-manager-quota-ledger"
	// ManagedBy marks the EndpointSlice as the ledger's, so the EndpointSlice controller leaves it alone.
	ManagedBy = "quota-ledger.multitenancy-manager.deckhouse.io"
	// Port is the port of the webhook server.
	Port = 9444
	// ProtocolVersion is part of the identity in the lease: a replica of another version of the ledger
	// can tell the holder is not of its own.
	ProtocolVersion = 1
)

// Self is the pod of this replica.
type Self struct {
	PodName string
	PodUID  types.UID
	PodIP   string
	RunID   string
}

// Identity is the identity of this run of the replica in the lease: a restarted container is a new
// holder and waits for the lease of its previous run to expire.
func (s Self) Identity() string {
	return fmt.Sprintf("%s_%s_v%d", s.PodName, s.RunID, ProtocolVersion)
}

// podOfIdentity returns the pod name of a lease identity.
func podOfIdentity(identity string) string {
	pod, _, _ := strings.Cut(identity, "_")
	return pod
}

// Endpoints writes the EndpointSlice of the ledger Service.
type Endpoints struct {
	Client  client.Client
	Self    Self
	Metrics *Metrics
}

// Publish makes the slice point at this replica alone, unless mayWrite, asked right before the write,
// says the replica may no longer write it. The slice is owned by the pod, so it goes when the pod is
// deleted.
func (e *Endpoints) Publish(ctx context.Context, mayWrite func() bool) error {
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: ServiceName}}
	err := e.Client.Get(ctx, client.ObjectKeyFromObject(slice), slice)
	if err == nil && e.pointsAtSelf(slice) {
		return nil
	}
	if (err == nil || apierrors.IsNotFound(err)) && !mayWrite() {
		return nil
	}
	switch {
	case apierrors.IsNotFound(err):
		e.fill(slice)
		if err := e.Client.Create(ctx, slice); err != nil {
			return fmt.Errorf("create the EndpointSlice of the ledger: %w", err)
		}
		e.written("leader")
		return nil
	case err != nil:
		return fmt.Errorf("get the EndpointSlice of the ledger: %w", err)
	}
	e.fill(slice)
	if err := e.Client.Update(ctx, slice); err != nil {
		return fmt.Errorf("update the EndpointSlice of the ledger: %w", err)
	}
	e.written("leader")
	return nil
}

// Withdraw takes the address of the pod out of the slice, if the slice points at it.
func (e *Endpoints) Withdraw(ctx context.Context, pod, actor string) error {
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: ServiceName}}
	switch err := e.Client.Get(ctx, client.ObjectKeyFromObject(slice), slice); {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("get the EndpointSlice of the ledger: %w", err)
	}
	kept := slice.Endpoints[:0]
	for _, endpoint := range slice.Endpoints {
		if endpoint.TargetRef == nil || endpoint.TargetRef.Name != pod {
			kept = append(kept, endpoint)
		}
	}
	if len(kept) == len(slice.Endpoints) {
		return nil
	}
	slice.Endpoints = kept
	if err := e.Client.Update(ctx, slice); err != nil {
		return fmt.Errorf("withdraw %s from the EndpointSlice of the ledger: %w", pod, err)
	}
	e.written(actor)
	return nil
}

func (e *Endpoints) written(actor string) {
	if e.Metrics != nil {
		e.Metrics.EndpointWrites.WithLabelValues(actor).Inc()
	}
}

func (e *Endpoints) pointsAtSelf(slice *discoveryv1.EndpointSlice) bool {
	return len(slice.Endpoints) == 1 && len(slice.Endpoints[0].Addresses) == 1 && slice.Endpoints[0].Addresses[0] == e.Self.PodIP &&
		slice.Endpoints[0].TargetRef != nil && slice.Endpoints[0].TargetRef.Name == e.Self.PodName
}

func (e *Endpoints) fill(slice *discoveryv1.EndpointSlice) {
	slice.Labels = map[string]string{
		discoveryv1.LabelServiceName: ServiceName,
		discoveryv1.LabelManagedBy:   ManagedBy,
	}
	slice.AddressType = discoveryv1.AddressTypeIPv4
	if strings.Contains(e.Self.PodIP, ":") {
		slice.AddressType = discoveryv1.AddressTypeIPv6
	}
	slice.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: e.Self.PodName, UID: e.Self.PodUID}}
	slice.Ports = []discoveryv1.EndpointPort{{Name: ptr.To("https"), Port: ptr.To[int32](Port), Protocol: ptr.To(corev1.ProtocolTCP)}}
	slice.Endpoints = []discoveryv1.Endpoint{{
		Addresses:  []string{e.Self.PodIP},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
		TargetRef:  &corev1.ObjectReference{Kind: "Pod", Namespace: Namespace, Name: e.Self.PodName, UID: e.Self.PodUID},
	}}
}

// watchdogWindow is how long a follower sees the lease unchanged before it takes the address of the
// holder out of the slice: the holder renews every second, so a lease that stands still for three
// is held by a process that is frozen, cut off, or lost with its node.
const watchdogWindow = 3 * time.Second

// Watchdog is the follower that clears the slice of a holder that stopped renewing, by its own
// monotonic clock and two successful reads in a row; the clocks of the masters are not compared.
type Watchdog struct {
	Reader    client.Reader
	Endpoints *Endpoints
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	last    string
	changed time.Time
	reads   int
}

func (w *Watchdog) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// Tick reads the lease once and withdraws the holder when the lease stood still for the window.
func (w *Watchdog) Tick(ctx context.Context) error {
	lease := new(coordinationv1.Lease)
	if err := w.Reader.Get(ctx, client.ObjectKey{Namespace: Namespace, Name: LeaseName}, lease); err != nil {
		w.reads = 0
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get the ledger lease: %w", err)
	}
	holder := ptr.Deref(lease.Spec.HolderIdentity, "")
	key := holder
	if lease.Spec.RenewTime != nil {
		key += "@" + lease.Spec.RenewTime.Format(time.RFC3339Nano)
	}
	now := w.now()
	if key != w.last {
		w.last, w.changed, w.reads = key, now, 1
		return nil
	}
	w.reads++
	if holder == "" || holder == w.Endpoints.Self.Identity() || w.reads < 2 || now.Sub(w.changed) < watchdogWindow {
		return nil
	}
	return w.Endpoints.Withdraw(ctx, podOfIdentity(holder), "watchdog")
}

// caughtUp reports whether the objects of an informer cover a quorum list: every listed object is
// there in its listed version or a newer one, and an object missing from the list is newer than the
// list, created after it rather than deleted before it.
func caughtUp(listed, cached []metav1.Object, listRV uint64) bool {
	versions := make(map[string]uint64, len(cached))
	for _, object := range cached {
		versions[object.GetNamespace()+"/"+object.GetName()] = ParseResourceVersion(object.GetResourceVersion())
	}
	inList := make(map[string]struct{}, len(listed))
	for _, object := range listed {
		key := object.GetNamespace() + "/" + object.GetName()
		inList[key] = struct{}{}
		v, ok := versions[key]
		if !ok || v < ParseResourceVersion(object.GetResourceVersion()) {
			return false
		}
	}
	for key, v := range versions {
		if _, ok := inList[key]; !ok && v <= listRV {
			return false
		}
	}
	return true
}
