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
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/event"
	"caps-controller-manager/internal/scope"
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

func newPendingStaticInstance(instanceLabels map[string]string) *deckhousev1.StaticInstance {
	return &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "static-instance",
			Labels: instanceLabels,
		},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{
				Phase: deckhousev1.StaticInstanceStatusCurrentStatusPhasePending,
			},
		},
	}
}

func newReconciler(t *testing.T, machines ...*infrav1.StaticMachine) *StaticMachineReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, machine := range machines {
		builder = builder.WithObjects(machine)
	}

	return &StaticMachineReconciler{Client: builder.Build()}
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

// newInstanceScopeFor wires a StaticInstance and its StaticMachine to a fake client, so that
// the phase handlers can patch them the way they do in the cluster.
func newInstanceScopeFor(
	t *testing.T,
	machine *infrav1.StaticMachine,
	instance *deckhousev1.StaticInstance,
) (*StaticMachineReconciler, *scope.InstanceScope) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(machine, instance).
		WithStatusSubresource(machine, instance).
		Build()

	instanceBaseScope, err := scope.NewScope(fakeClient, &rest.Config{}, logr.Discard())
	require.NoError(t, err)

	instanceScope, err := scope.NewInstanceScope(instanceBaseScope, instance, context.Background())
	require.NoError(t, err)

	machineBaseScope, err := scope.NewScope(fakeClient, &rest.Config{}, logr.Discard())
	require.NoError(t, err)

	machineBaseScope.PatchHelper, err = patch.NewHelper(machine, fakeClient)
	require.NoError(t, err)

	instanceScope.AttachMachineScope(&scope.MachineScope{Scope: machineBaseScope, StaticMachine: machine})

	reconciler := &StaticMachineReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: event.NewRecorder(fakeClient, logr.Discard()),
	}

	return reconciler, instanceScope
}

// ProviderID is assigned only after both connectivity checks pass, so an empty one means the
// host was never touched. Such an instance must go back to the pool on the bootstrap timeout
// instead of waiting for MachineHealthCheck remediation and the cleanup timeout.
func TestBootstrapTimeoutReleasesInstanceWhenHostWasNeverTouched(t *testing.T) {
	machine := newStaticMachine("machine", "worker")
	machine.UID = types.UID("machine-uid")
	instance := newReservedStaticInstance(machine, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, DefaultStaticInstanceBootstrapTimeout+time.Minute)

	reconciler, instanceScope := newInstanceScopeFor(t, machine, instance)

	_, err := reconciler.reconcileStaticInstancePhase(context.Background(), instanceScope)

	require.ErrorIs(t, err, ErrStaticMachineBootstrapTimedOut)
	require.NotNil(t, machine.Status.FailureReason)
	require.Equal(t, "CreateError", *machine.Status.FailureReason)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, instanceScope.GetPhase())
	require.Nil(t, instanceScope.Instance.Status.MachineRef)
}

// A host that already ran the bootstrap script may be half-configured, so it must not be handed
// to another StaticMachine. The reservation is held and the MachineHealthCheck path, which runs
// the remote cleanup, is the only way back.
func TestBootstrapTimeoutKeepsReservationWhenHostWasTouched(t *testing.T) {
	machine := newStaticMachine("machine", "worker")
	machine.UID = types.UID("machine-uid")
	machine.Spec.ProviderID = "static://static-instance"
	instance := newReservedStaticInstance(machine, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, DefaultStaticInstanceBootstrapTimeout+time.Minute)

	reconciler, instanceScope := newInstanceScopeFor(t, machine, instance)

	_, err := reconciler.reconcileStaticInstancePhase(context.Background(), instanceScope)

	require.ErrorIs(t, err, ErrStaticMachineBootstrapTimedOut)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, instanceScope.GetPhase())
	require.NotNil(t, instanceScope.Instance.Status.MachineRef)
}

// The remote cleanup script wipes /var/lib/bashible and reboots the host. Running it on a host
// caps never reached would reboot a machine an administrator is only preparing, so the delete
// flow must skip it and release the instance directly.
func TestCleanupSkipsRemoteScriptWhenHostWasNeverTouched(t *testing.T) {
	machine := newStaticMachine("machine", "worker")
	machine.UID = types.UID("machine-uid")
	instance := newReservedStaticInstance(machine, deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, time.Minute)

	reconciler, instanceScope := newInstanceScopeFor(t, machine, instance)

	// HostClient stays nil on purpose: reaching it would mean the remote cleanup was attempted.
	result, err := reconciler.cleanup(context.Background(), instanceScope)

	require.NoError(t, err)
	require.Equal(t, RequeueForStaticMachineDeleting, result.RequeueAfter)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, instanceScope.GetPhase())
	require.Nil(t, instanceScope.Instance.Status.MachineRef)
}

// A StaticInstance going back to Pending must only wake up the StaticMachines that could
// actually consume it. Enqueueing every non-ready StaticMachine turns one release into a
// cluster-wide reconcile burst.
func TestStaticInstanceToStaticMachineMapFuncMatchesLabelSelectorOnly(t *testing.T) {
	matching := newStaticMachine("matching", "worker")
	nonMatching := newStaticMachine("non-matching", "system")

	ready := newStaticMachine("ready", "worker")
	ready.Status.Ready = true

	provisioned := newStaticMachine("provisioned", "worker")
	isProvisioned := true
	provisioned.Status.Initialization.Provisioned = &isProvisioned

	second := newStaticMachine("second", "worker")

	reconciler := newReconciler(t, matching, nonMatching, ready, provisioned, second)

	instance := newPendingStaticInstance(map[string]string{"node-group": "worker"})

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
	reconciler := newReconciler(t, newStaticMachine("matching", "worker"))

	instance := newPendingStaticInstance(map[string]string{
		"node-group":                "worker",
		infrav1.AllowBootstrapLabel: "false",
	})

	mapFunc := reconciler.StaticInstanceToStaticMachineMapFunc(infrav1.GroupVersion.WithKind("StaticMachine"))

	require.Empty(t, mapFunc(context.Background(), instance))
}
