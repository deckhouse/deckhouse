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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

func pendingStaticInstance() *deckhousev1.StaticInstance {
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

func staticMachine() *infrav1.StaticMachine {
	return &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "static-machine",
			Namespace: "d8-cloud-instance-manager",
			UID:       types.UID("a2c0f1ba-0000-4000-8000-000000000001"),
		},
	}
}

// A failed bootstrap attempt must leave the StaticInstance exactly as it was fetched.
// Any diff here becomes a write to etcd and a Pending watch event that re-enqueues every
// matching StaticMachine at once.
func TestReserveReleaseRoundTripProducesNoDiff(t *testing.T) {
	c := &Client{}
	instance := pendingStaticInstance()
	machine := staticMachine()

	roundTrip := func() {
		observedStatus := instance.Status.CurrentStatus.DeepCopy()

		require.NoError(t, c.reserveStaticInstance(instance, machine))
		require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, instance.GetPhase())
		require.NotNil(t, instance.Status.MachineRef)

		c.releaseStaticInstance(instance, machine, observedStatus)
	}

	// The first attempt legitimately records the failure condition.
	roundTrip()

	settled := instance.DeepCopy()

	// Every attempt after that must be a no-op for the API server.
	roundTrip()

	require.Equal(t, settled.Status, instance.Status)
}

// When the instance was already Bootstrapping, releasing it has to move it back to Pending
// with a fresh timestamp: restoring the snapshot would leave a reserved phase without a
// machineRef, and the instance would never be picked from the pool again.
func TestReleaseFromBootstrappingFallsBackToPending(t *testing.T) {
	c := &Client{}
	machine := staticMachine()

	instance := pendingStaticInstance()
	require.NoError(t, c.reserveStaticInstance(instance, machine))

	observedStatus := instance.Status.CurrentStatus.DeepCopy()

	c.releaseStaticInstance(instance, machine, observedStatus)

	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, instance.GetPhase())
	require.Nil(t, instance.Status.MachineRef)
}

func TestReleaseIgnoresInstanceReservedByAnotherMachine(t *testing.T) {
	c := &Client{}
	instance := pendingStaticInstance()

	owner := staticMachine()
	require.NoError(t, c.reserveStaticInstance(instance, owner))

	other := staticMachine()
	other.UID = types.UID("a2c0f1ba-0000-4000-8000-000000000002")

	reserved := instance.DeepCopy()

	c.releaseStaticInstance(instance, other, nil)

	require.Equal(t, reserved.Status, instance.Status)
}
