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

package fencingfailednodestate

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

const nodeName = "worker-3"

var (
	detectedAt   = metav1.Date(2026, time.June, 2, 15, 0, 1, 0, time.UTC)
	quorumLostAt = metav1.Date(2026, time.June, 2, 15, 0, 0, 0, time.UTC)
	heartbeatAt  = metav1.Date(2026, time.June, 2, 15, 0, 2, 0, time.UTC)
)

func TestReconcileObservesFailedStateWithoutTouchingIt(t *testing.T) {
	state := failedState()

	c := newClient(t, state)

	res, err := New(c).Reconcile(t.Context(), request(nodeName))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v, this stage reacts to watch events only", res)
	}

	assertUnchanged(t, c, state)
}

func TestReconcileObservesEveryStatusShape(t *testing.T) {
	both := failedState()
	both.Status.Phase = v1alpha1.PhaseFallbackAlive
	both.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{
		Active:                   true,
		LastHeartbeatAt:          &heartbeatAt,
		QuorumLostAt:             &quorumLostAt,
		HeartbeatIntervalSeconds: 1,
	}

	none := failedState()
	none.Status = v1alpha1.FencingFailedNodeStateStatus{}

	for name, state := range map[string]*v1alpha1.FencingFailedNodeState{
		"failed and fallback": both,
		"status not written":  none,
	} {
		t.Run(name, func(t *testing.T) {
			c := newClient(t, state)

			if _, err := New(c).Reconcile(t.Context(), request(nodeName)); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			assertUnchanged(t, c, state)
		})
	}
}

func TestReconcileTreatsMissingObjectAsHealthy(t *testing.T) {
	res, err := New(newClient(t)).Reconcile(t.Context(), request(nodeName))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v for a missing object", res)
	}
}

func TestReconcileReturnsAPIErrors(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return apiDown
			},
		}).
		Build()

	if _, err := New(c).Reconcile(t.Context(), request(nodeName)); !errors.Is(err, apiDown) {
		t.Fatalf("reconcile returned %v, want the API error so controller-runtime retries with backoff", err)
	}
}

func TestObservedFieldsAreBalancedPairs(t *testing.T) {
	deleting := failedState()
	deleting.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{Active: true}
	deleting.DeletionTimestamp = &heartbeatAt

	for name, state := range map[string]*v1alpha1.FencingFailedNodeState{
		"every section set": deleting,
		"zero value":        {},
	} {
		t.Run(name, func(t *testing.T) {
			fields := observedFields(state)

			if len(fields)%2 != 0 {
				t.Fatalf("observedFields returned %d values, want key-value pairs", len(fields))
			}

			for i := 0; i < len(fields); i += 2 {
				if _, ok := fields[i].(string); !ok {
					t.Errorf("key at index %d is %T, want string", i, fields[i])
				}
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	for name, tc := range map[string]struct {
		in   *metav1.Time
		want string
	}{
		"unset":      {in: nil, want: ""},
		"zero value": {in: &metav1.Time{}, want: ""},
		"set":        {in: &detectedAt, want: "2026-06-02T15:00:01Z"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := formatTime(tc.in); got != tc.want {
				t.Errorf("formatTime = %q, want %q", got, tc.want)
			}
		})
	}
}

func failedState() *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:       nodeName,
			Generation: 1,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Node",
				Name:       nodeName,
				UID:        types.UID("1a2b3c4d-1111-2222-3333-444455556666"),
			}},
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			NodeGroup:  "worker",
			ProfileRef: v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Phase: v1alpha1.PhaseSuspected,
			Failed: &v1alpha1.FencingFailedNodeStateFailed{
				DetectedAt: detectedAt,
				DetectedBy: "worker-1",
				Reason:     v1alpha1.FailedReasonMemberlistDead,
				AliveCount: 3,
				QuorumSize: 3,
			},
		},
	}
}

func newClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()

	reject := func(verb string) error {
		t.Errorf("reconcile issued %s, this stage must not write to the cluster", verb)

		return nil
	}

	return fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.FencingFailedNodeState{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
				return reject("create")
			},
			Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
				return reject("update")
			},
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				return reject("patch")
			},
			Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
				return reject("delete")
			},
			DeleteAllOf: func(context.Context, client.WithWatch, client.Object, ...client.DeleteAllOfOption) error {
				return reject("deleteAllOf")
			},
			SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
				return reject("status update")
			},
			SubResourcePatch: func(context.Context, client.Client, string, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
				return reject("status patch")
			},
		}).
		Build()
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add fencing API to scheme: %v", err)
	}

	return s
}

func assertUnchanged(t *testing.T, c client.Client, want *v1alpha1.FencingFailedNodeState) {
	t.Helper()

	var got v1alpha1.FencingFailedNodeState
	if err := c.Get(t.Context(), types.NamespacedName{Name: want.Name}, &got); err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}

	if got.ResourceVersion != want.ResourceVersion {
		t.Errorf("resourceVersion changed from %q to %q", want.ResourceVersion, got.ResourceVersion)
	}

	if !equality.Semantic.DeepEqual(want, &got) {
		t.Errorf("object changed:\nbefore %+v\nafter  %+v", want, &got)
	}
}

func request(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}
