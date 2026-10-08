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
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/event"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
)

const instanceNamespace = "d8-cloud-instance-manager"

// SSHCredentials are cluster-scoped and the reference is mandatory on the CRD, so every object
// built here carries one. The controller only has to be able to load it, whether it exists or not.
const testCredentialsName = "test-credentials"

func orphanTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, infrav1.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))

	return scheme
}

func instanceWithMachineRef(name string, phase deckhousev1.StaticInstanceStatusCurrentStatusPhase, machineName string) *deckhousev1.StaticInstance {
	instance := &deckhousev1.StaticInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.StaticInstanceSpec{
			Address:        "192.0.2.1",
			CredentialsRef: &corev1.ObjectReference{Name: testCredentialsName},
		},
		Status: deckhousev1.StaticInstanceStatus{
			CurrentStatus: &deckhousev1.StaticInstanceStatusCurrentStatus{Phase: phase},
		},
	}

	if machineName != "" {
		instance.Status.MachineRef = &corev1.ObjectReference{
			APIVersion: infrav1.GroupVersion.String(),
			Kind:       "StaticMachine",
			Namespace:  instanceNamespace,
			Name:       machineName,
			UID:        types.UID(machineName + "-uid"),
		}
	}

	return instance
}

func newInstanceReconciler(t *testing.T, objects ...client.Object) (*StaticInstanceReconciler, client.Client) {
	t.Helper()

	scheme := orphanTestScheme(t)

	// The credentials reference is mandatory, so the object has to be resolvable for a reconcile
	// to get past the credential check and reach the phase handling under test.
	credentials := &deckhousev1.SSHCredentials{ObjectMeta: metav1.ObjectMeta{Name: testCredentialsName}}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(append([]client.Object{credentials}, objects...)...).
		WithStatusSubresource(&deckhousev1.StaticInstance{}).
		Build()

	return &StaticInstanceReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: event.NewRecorder(fakeClient, logr.Discard()),
	}, fakeClient
}

func reconcileInstance(t *testing.T, reconciler *StaticInstanceReconciler, name string) error {
	t.Helper()

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: name},
	})

	return err
}

func getInstance(t *testing.T, cli client.Client, name string) *deckhousev1.StaticInstance {
	t.Helper()

	instance := &deckhousev1.StaticInstance{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKey{Name: name}, instance))

	return instance
}

// A StaticInstance is handed back to the pool only by the StaticMachine delete flow. When the
// referenced StaticMachine is already gone that flow will never run again, so the instance has to
// be released by the controller that notices the dangling reference - otherwise it keeps a
// machineRef, stays out of Pending, and is invisible to PickStaticInstance forever.
func TestReconcileReleasesInstanceWhoseStaticMachineIsGone(t *testing.T) {
	for _, phase := range []deckhousev1.StaticInstanceStatusCurrentStatusPhase{
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping,
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning,
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning,
	} {
		t.Run(string(phase), func(t *testing.T) {
			instance := instanceWithMachineRef("orphan", phase, "gone-machine")
			instance.Status.NodeRef = &corev1.ObjectReference{Name: "node1"}

			reconciler, fakeClient := newInstanceReconciler(t, instance)

			require.NoError(t, reconcileInstance(t, reconciler, instance.Name))

			updated := getInstance(t, fakeClient, instance.Name)
			require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, updated.GetPhase())
			require.Nil(t, updated.Status.MachineRef)
			require.Nil(t, updated.Status.NodeRef)
		})
	}
}

// An instance that is already Pending has nothing to release and must not be rewritten: every
// write produces a watch event and a reconcile, which is the self-sustaining loop the caps
// controllers are careful to avoid.
func TestReconcileLeavesPendingInstanceAlone(t *testing.T) {
	instance := instanceWithMachineRef("pending", deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, "")

	reconciler, fakeClient := newInstanceReconciler(t, instance)

	require.NoError(t, reconcileInstance(t, reconciler, instance.Name))

	updated := getInstance(t, fakeClient, instance.Name)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhasePending, updated.GetPhase())
	require.Nil(t, updated.Status.MachineRef)
}

// The release is driven by "the object is not there", never by "could not ask". A transient API
// error must leave the instance untouched so the retry can still find a healthy StaticMachine.
func TestReconcileDoesNotReleaseInstanceOnFailedGet(t *testing.T) {
	instance := instanceWithMachineRef("flaky", deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning, "some-machine")

	reconciler, fakeClient := newInstanceReconciler(t, instance)
	reconciler.Client = failingStaticMachineClient{Client: fakeClient}

	err := reconcileInstance(t, reconciler, instance.Name)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get StaticMachine")

	updated := getInstance(t, fakeClient, instance.Name)
	require.Equal(t, deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning, updated.GetPhase())
	require.NotNil(t, updated.Status.MachineRef)
	require.Equal(t, "some-machine", updated.Status.MachineRef.Name)
}

type failingStaticMachineClient struct {
	client.Client
}

func (c failingStaticMachineClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*infrav1.StaticMachine); ok {
		return errors.New("connection refused")
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

// The release is only as timely as the event that triggers it. Waiting for the periodic resync
// would miss the scale-up right after the scale-down that orphaned the instance.
func TestStaticMachineToStaticInstanceMapFuncEnqueuesBoundInstances(t *testing.T) {
	scheme := orphanTestScheme(t)

	bound := instanceWithMachineRef("bound", deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, "machine")
	other := instanceWithMachineRef("other", deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning, "another-machine")

	uid := types.UID("machine-uid")
	bound.Status.MachineRef.UID = uid

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(bound, other).
		WithIndex(&deckhousev1.StaticInstance{}, "status.machineRef.uid", func(rawObj client.Object) []string {
			instance := rawObj.(*deckhousev1.StaticInstance)
			if instance.Status.MachineRef == nil {
				return nil
			}

			return []string{string(instance.Status.MachineRef.UID)}
		}).
		Build()

	reconciler := &StaticInstanceReconciler{Client: fakeClient, Scheme: scheme}

	staticMachine := &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "machine", Namespace: instanceNamespace, UID: uid},
	}

	requests := reconciler.staticMachineToStaticInstanceMapFunc()(context.Background(), staticMachine)

	require.Len(t, requests, 1)
	require.Equal(t, "bound", requests[0].Name)
	require.Empty(t, requests[0].Namespace)
}

func TestStaticMachineToStaticInstanceMapFuncSkipsUnknownUID(t *testing.T) {
	reconciler, _ := newInstanceReconciler(t)

	staticMachine := &infrav1.StaticMachine{
		ObjectMeta: metav1.ObjectMeta{Name: "machine", Namespace: instanceNamespace},
	}

	require.Empty(t, reconciler.staticMachineToStaticInstanceMapFunc()(context.Background(), staticMachine))
}
