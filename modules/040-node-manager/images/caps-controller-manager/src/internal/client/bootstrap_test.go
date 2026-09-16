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

package client

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/scope"
)

const testNamespace = "d8-cloud-instance-manager"

func newPendingStaticInstance() *deckhousev1.StaticInstance {
	return &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "static-instance"},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-time.Hour).UTC()),
			},
		},
	}
}

func newStaticMachine(uid string) *infrav1.StaticMachine {
	return &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "static-machine",
			Namespace: testNamespace,
			UID:       types.UID(uid),
		},
	}
}

func newInstanceScope(t *testing.T, instance *deckhousev1.StaticInstance, machine *infrav1.StaticMachine) *scope.InstanceScope {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, deckhousev1.AddToScheme(scheme))
	require.NoError(t, infrav1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance, machine).
		WithStatusSubresource(instance, machine).
		Build()

	baseScope, err := scope.NewScope(fakeClient, &rest.Config{}, logr.Discard())
	require.NoError(t, err)

	instanceScope, err := scope.NewInstanceScope(baseScope, instance, context.Background())
	require.NoError(t, err)

	machineScope, err := scope.NewScope(fakeClient, &rest.Config{}, logr.Discard())
	require.NoError(t, err)

	instanceScope.AttachMachineScope(&scope.MachineScope{Scope: machineScope, StaticMachine: machine})

	return instanceScope
}

// A failed bootstrap attempt must leave the StaticInstance exactly as it was fetched.
// Any diff here becomes a write to etcd and a Pending watch event that re-enqueues every
// matching StaticMachine at once.
func TestReserveReleaseRoundTripProducesNoDiff(t *testing.T) {
	ctx := context.Background()

	c := &Client{}
	instance := newPendingStaticInstance()
	machine := newStaticMachine("a2c0f1ba-0000-4000-8000-000000000001")
	instanceScope := newInstanceScope(t, instance, machine)

	roundTrip := func() {
		observedStatus := instanceScope.Instance.Status.CurrentStatus.DeepCopy()

		require.NoError(t, c.reserveStaticInstance(ctx, instanceScope))
		require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, instanceScope.GetPhase())
		require.NotNil(t, instanceScope.Instance.Status.MachineRef)

		c.releaseStaticInstance(ctx, instanceScope, observedStatus)
	}

	// The first attempt legitimately records the failure condition.
	roundTrip()

	settled := instanceScope.Instance.DeepCopy()

	// Every attempt after that must be a no-op for the API server.
	roundTrip()

	require.Equal(t, settled.Status, instanceScope.Instance.Status)
}

// When the instance was already Bootstrapping, releasing it has to move it back to Pending
// with a fresh timestamp: restoring the snapshot would leave a reserved phase without a
// machineRef, and the instance would never be picked from the pool again.
func TestReleaseFromBootstrappingFallsBackToPending(t *testing.T) {
	ctx := context.Background()

	c := &Client{}
	instance := newPendingStaticInstance()
	machine := newStaticMachine("a2c0f1ba-0000-4000-8000-000000000001")
	instanceScope := newInstanceScope(t, instance, machine)

	require.NoError(t, c.reserveStaticInstance(ctx, instanceScope))

	observedStatus := instanceScope.Instance.Status.CurrentStatus.DeepCopy()

	c.releaseStaticInstance(ctx, instanceScope, observedStatus)

	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, instanceScope.GetPhase())
	require.Nil(t, instanceScope.Instance.Status.MachineRef)
}

func TestReleaseIgnoresInstanceReservedByAnotherMachine(t *testing.T) {
	ctx := context.Background()

	c := &Client{}
	instance := newPendingStaticInstance()
	owner := newStaticMachine("a2c0f1ba-0000-4000-8000-000000000001")
	instanceScope := newInstanceScope(t, instance, owner)

	require.NoError(t, c.reserveStaticInstance(ctx, instanceScope))

	reserved := instanceScope.Instance.DeepCopy()

	other := newStaticMachine("a2c0f1ba-0000-4000-8000-000000000002")
	instanceScope.MachineScope.StaticMachine = other

	c.releaseStaticInstance(ctx, instanceScope, nil)

	require.Equal(t, reserved.Status, instanceScope.Instance.Status)
}
