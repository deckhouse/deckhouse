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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/event"
)

const testNamespace = "d8-cloud-instance-manager"

func newStaticMachine(name string, nodeGroup string) *infrav1.StaticMachine {
	return &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace},
		Spec: infrav1.StaticMachineSpec{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"node-group": nodeGroup},
			},
		},
	}
}

func newReservedStaticInstance(machine *infrav1.StaticMachine, phase deckhousev1.StaticInstanceStatusCurrentStatusPhase, phaseAge time.Duration) *deckhousev1.StaticInstance {
	return &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "static-instance"},
		Spec:       deckhousev1.StaticInstanceSpec{Address: "192.0.2.1"},
		Status: deckhousev1.StaticInstanceStatus{
			MachineRef: &corev1.ObjectReference{
				Name:      machine.Name,
				Namespace: machine.Namespace,
				UID:       machine.UID,
			},
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase:          phase,
				LastUpdateTime: metav1.NewTime(time.Now().Add(-phaseAge)),
			},
		},
	}
}

func newTestReconciler(t *testing.T) *StaticMachineReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	return &StaticMachineReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: event.NewRecorder(fakeClient, logr.Discard()),
	}
}

// ProviderID is assigned only after both connectivity checks pass, so an empty one means the
// host was never touched. Such an instance must go back to the pool on the bootstrap timeout
// instead of waiting for MachineHealthCheck remediation and the cleanup timeout.
func TestBootstrapTimeoutReleasesInstanceWhenHostWasNeverTouched(t *testing.T) {
	reconciler := newTestReconciler(t)

	staticMachine := newStaticMachine("machine", "worker")
	staticMachine.UID = types.UID("machine-uid")
	staticInstance := newReservedStaticInstance(staticMachine, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, DefaultStaticInstanceBootstrapTimeout+time.Minute)

	_, err := reconciler.reconcileStaticInstancePhase(context.Background(), &clusterv1.Machine{}, staticMachine, staticInstance)

	require.ErrorIs(t, err, ErrStaticMachineBootstrapTimedOut)
	require.Equal(t, "CreateError", ptr.Deref(staticMachine.Status.FailureReason, ""))
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, staticInstance.GetPhase())
	require.Nil(t, staticInstance.Status.MachineRef)
}

// A host that already ran the bootstrap script may be half-configured, so it must not be handed
// to another StaticMachine. The reservation is held and the MachineHealthCheck path, which runs
// the remote cleanup, is the only way back.
func TestBootstrapTimeoutKeepsReservationWhenHostWasTouched(t *testing.T) {
	reconciler := newTestReconciler(t)

	staticMachine := newStaticMachine("machine", "worker")
	staticMachine.UID = types.UID("machine-uid")
	staticMachine.Spec.ProviderID = "static://static-instance"
	staticInstance := newReservedStaticInstance(staticMachine, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, DefaultStaticInstanceBootstrapTimeout+time.Minute)

	_, err := reconciler.reconcileStaticInstancePhase(context.Background(), &clusterv1.Machine{}, staticMachine, staticInstance)

	require.ErrorIs(t, err, ErrStaticMachineBootstrapTimedOut)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, staticInstance.GetPhase())
	require.NotNil(t, staticInstance.Status.MachineRef)
}

// The remote cleanup script wipes /var/lib/bashible and reboots the host. Running it on a host
// caps never reached would reboot a machine an administrator is only preparing, so the delete
// flow must skip it and release the instance directly.
func TestCleanupSkipsRemoteScriptWhenHostWasNeverTouched(t *testing.T) {
	reconciler := newTestReconciler(t)

	staticMachine := newStaticMachine("machine", "worker")
	staticMachine.UID = types.UID("machine-uid")
	staticInstance := newReservedStaticInstance(staticMachine, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, time.Minute)

	// HostClient stays nil on purpose: reaching it would mean the remote cleanup was attempted.
	result, err := reconciler.cleanup(context.Background(), &clusterv1.Machine{}, staticMachine, staticInstance)

	require.NoError(t, err)
	require.Equal(t, RequeueForStaticMachineDeleting, result.RequeueAfter)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, staticInstance.GetPhase())
	require.Nil(t, staticInstance.Status.MachineRef)
}

// A StaticInstance going back to Pending must only wake up the StaticMachines that could
// actually consume it. Enqueueing every non-ready StaticMachine turns one release into a
// cluster-wide reconcile burst.
func TestStaticInstanceToStaticMachineMapFuncMatchesLabelSelectorOnly(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	matching := newStaticMachine("matching", "worker")
	nonMatching := newStaticMachine("non-matching", "system")

	ready := newStaticMachine("ready", "worker")
	ready.Status.Ready = true

	provisioned := newStaticMachine("provisioned", "worker")
	provisioned.Status.Initialization.Provisioned = ptr.To(true)

	second := newStaticMachine("second", "worker")

	reconciler := &StaticMachineReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(matching, nonMatching, ready, provisioned, second).
			Build(),
	}

	instance := &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "static-instance",
			Labels: map[string]string{
				"node-group": "worker",
			},
		},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase: deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
			},
		},
	}

	mapFunc := reconciler.StaticInstanceToStaticMachineMapFunc(infrav1.GroupVersion.WithKind("StaticMachine"))
	requests := mapFunc(context.Background(), instance)

	require.Len(t, requests, 2)

	names := make([]string, 0, len(requests))
	for _, request := range requests {
		require.Equal(t, testNamespace, request.Namespace)
		names = append(names, request.Name)
	}

	require.ElementsMatch(t, []string{"matching", "second"}, names)
}

// StaticInstances excluded from bootstrapping must not wake anyone up.
func TestStaticInstanceToStaticMachineMapFuncSkipsBootstrapDisabledInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	reconciler := &StaticMachineReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(newStaticMachine("matching", "worker")).
			Build(),
	}

	instance := &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "static-instance",
			Labels: map[string]string{
				"node-group":                        "worker",
				"node.deckhouse.io/allow-bootstrap": "false",
			},
		},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase: deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
			},
		},
	}

	mapFunc := reconciler.StaticInstanceToStaticMachineMapFunc(infrav1.GroupVersion.WithKind("StaticMachine"))

	require.Empty(t, mapFunc(context.Background(), instance))
}
