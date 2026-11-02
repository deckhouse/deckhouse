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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, discoveryv1.AddToScheme, coordinationv1.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func slice(t *testing.T, c client.Client) *discoveryv1.EndpointSlice {
	t.Helper()
	s := new(discoveryv1.EndpointSlice)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Namespace: Namespace, Name: ServiceName}, s))
	return s
}

var (
	podA = Self{PodName: "mtm-a", PodUID: "uid-a", PodIP: "10.1.0.5", RunID: "r1"}
	podB = Self{PodName: "mtm-b", PodUID: "uid-b", PodIP: "10.1.0.6", RunID: "r2"}
)

func TestPublishAndWithdraw(t *testing.T) {
	c := newClient(t)
	a := &Endpoints{Client: c, Self: podA}
	require.NoError(t, a.Publish(context.Background(), always))

	s := slice(t, c)
	assert.Equal(t, ServiceName, s.Labels[discoveryv1.LabelServiceName])
	assert.Equal(t, ManagedBy, s.Labels[discoveryv1.LabelManagedBy])
	require.Len(t, s.Endpoints, 1)
	assert.Equal(t, []string{"10.1.0.5"}, s.Endpoints[0].Addresses)
	assert.Equal(t, "mtm-a", s.OwnerReferences[0].Name, "the slice goes with the pod of the leader")
	assert.Equal(t, int32(Port), *s.Ports[0].Port)

	version := s.ResourceVersion
	require.NoError(t, a.Publish(context.Background(), always))
	assert.Equal(t, version, slice(t, c).ResourceVersion, "an unchanged slice is not written")

	b := &Endpoints{Client: c, Self: podB}
	require.NoError(t, b.Withdraw(context.Background(), "mtm-b", "leader"))
	assert.Len(t, slice(t, c).Endpoints, 1, "b withdraws only itself")
	require.NoError(t, b.Publish(context.Background(), always))
	assert.Equal(t, []string{"10.1.0.6"}, slice(t, c).Endpoints[0].Addresses)
	require.NoError(t, a.Withdraw(context.Background(), "mtm-b", "watchdog"))
	assert.Empty(t, slice(t, c).Endpoints)
}

// A replica that lost the right to write while it read the slice leaves the slice as it is: a leader
// resumed after a pause must not put its address back over the one of the new leader.
func TestPublishOnlyWhileItMayWrite(t *testing.T) {
	c := newClient(t)
	require.NoError(t, (&Endpoints{Client: c, Self: podB}).Publish(context.Background(), always))
	version := slice(t, c).ResourceVersion

	asked := 0
	stale := func() bool {
		asked++
		return false
	}
	require.NoError(t, (&Endpoints{Client: c, Self: podA}).Publish(context.Background(), stale))
	assert.Equal(t, 1, asked, "asked after the read, right before the write")
	assert.Equal(t, version, slice(t, c).ResourceVersion)
	assert.Equal(t, []string{"10.1.0.6"}, slice(t, c).Endpoints[0].Addresses)
}

func lease(holder string, renew time.Time) *coordinationv1.Lease {
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: LeaseName},
		Spec:       coordinationv1.LeaseSpec{HolderIdentity: ptr.To(holder), RenewTime: &metav1.MicroTime{Time: renew}},
	}
}

// A follower takes the address of a holder whose lease stood still for the window out of the slice,
// by its own clock, and leaves a renewing holder alone.
func TestWatchdog(t *testing.T) {
	renew := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	c := newClient(t, lease(podA.Identity(), renew))
	require.NoError(t, (&Endpoints{Client: c, Self: podA}).Publish(context.Background(), always))

	clock := time.Now()
	w := &Watchdog{Reader: c, Endpoints: &Endpoints{Client: c, Self: podB}, Now: func() time.Time { return clock }}
	ctx := context.Background()
	require.NoError(t, w.Tick(ctx))

	clock = clock.Add(2 * time.Second)
	require.NoError(t, w.Tick(ctx))
	assert.Len(t, slice(t, c).Endpoints, 1, "2 s is within the window")

	// the holder renews: the window starts again
	l := new(coordinationv1.Lease)
	require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: Namespace, Name: LeaseName}, l))
	l.Spec.RenewTime = &metav1.MicroTime{Time: renew.Add(time.Second)}
	require.NoError(t, c.Update(ctx, l))
	clock = clock.Add(2 * time.Second)
	require.NoError(t, w.Tick(ctx))
	assert.Len(t, slice(t, c).Endpoints, 1)

	clock = clock.Add(3 * time.Second)
	require.NoError(t, w.Tick(ctx))
	assert.Empty(t, slice(t, c).Endpoints, "the lease stood still for 3 s")
}

func TestWatchdogLeavesItsOwnLease(t *testing.T) {
	c := newClient(t, lease(podB.Identity(), time.Now()))
	require.NoError(t, (&Endpoints{Client: c, Self: podB}).Publish(context.Background(), always))
	clock := time.Now()
	w := &Watchdog{Reader: c, Endpoints: &Endpoints{Client: c, Self: podB}, Now: func() time.Time { return clock }}
	for range 5 {
		require.NoError(t, w.Tick(context.Background()))
		clock = clock.Add(2 * time.Second)
	}
	assert.Len(t, slice(t, c).Endpoints, 1)
}

func object(name, rv string) metav1.Object {
	return &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Namespace: name, Name: "d8-project-quota", ResourceVersion: rv}}
}

func TestCaughtUp(t *testing.T) {
	listed := []metav1.Object{object("a", "10"), object("b", "12")}
	tests := []struct {
		name     string
		cached   []metav1.Object
		expected bool
	}{
		{name: "the same", cached: []metav1.Object{object("a", "10"), object("b", "12")}, expected: true},
		{name: "newer", cached: []metav1.Object{object("a", "15"), object("b", "12")}, expected: true},
		{name: "the informer lags behind", cached: []metav1.Object{object("a", "9"), object("b", "12")}},
		{name: "the informer misses an object", cached: []metav1.Object{object("a", "10")}},
		{name: "created after the list", cached: []metav1.Object{object("a", "10"), object("b", "12"), object("c", "21")}, expected: true},
		{name: "deleted before the list", cached: []metav1.Object{object("a", "10"), object("b", "12"), object("c", "11")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, caughtUp(listed, tt.cached, 20))
		})
	}
}

func TestIdentity(t *testing.T) {
	assert.Equal(t, "mtm-a_r1_v1", podA.Identity())
	assert.Equal(t, "mtm-a", podOfIdentity(podA.Identity()))
}
