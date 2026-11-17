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

package dexauthenticator

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
)

// statusServer is a pod of an instance: it answers the status path with body over TLS with a
// certificate nobody signed, as a pod does.
func statusServer(t *testing.T, calls *atomic.Int64, status int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != statusPath || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func instancePod(name, instance, ip string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: naming.DexNamespace,
			Labels: map[string]string{InstanceAppLabel: InstanceAppValue, InstanceLabel: instance},
		},
		Status: corev1.PodStatus{
			Phase: phase, PodIP: ip,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readyStatus(phase == corev1.PodRunning)}},
		},
	}
}

func readyStatus(ready bool) corev1.ConditionStatus {
	if ready {
		return corev1.ConditionTrue
	}
	return corev1.ConditionFalse
}

// notReady returns the pod with its Ready condition off.
func notReady(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
	return pod
}

// newTestPodStatus returns a podStatus that takes the address of a pod from its IP: the tests
// put the URL of a test server there.
func newTestPodStatus(t *testing.T, pods ...client.Object) *podStatus {
	t.Helper()

	status, _ := NewPodStatus(newFakeClient(t, pods...)).(*podStatus)
	status.url = func(ip string) string { return ip + statusPath }
	return status
}

func TestPodStatus_OldestGenerationOfTheRunningPods(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	broken := &Record{Namespace: "shop", Name: "broken", CookieSecret: "dh-test-secret-0123456789abcdef1"}
	refusedOther := &Record{Namespace: "shop", Name: "other", CookieSecret: "dh-test-secret-0123456789abcdef2"}
	served := &Record{Namespace: "shop", Name: "app", CookieSecret: "dh-test-secret-0123456789abcdef0"}
	ahead := statusServer(t, &calls, http.StatusOK, `{"generation":7,"clients":3,"failed":["`+statusID(broken)+`"]}`)
	behind := statusServer(t, &calls, http.StatusOK, `{"generation":5,"clients":2,"failed":["`+statusID(broken)+`","`+statusID(refusedOther)+`"]}`)
	other := statusServer(t, &calls, http.StatusOK, `{"generation":1,"clients":0,"failed":[]}`)

	deleted := instancePod("deleted", naming.UserAuthenticatorInstance, "https://127.0.0.1:1", corev1.PodRunning)
	deleted.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	deleted.Finalizers = []string{"test/keep"}
	status := newTestPodStatus(t,
		instancePod("ahead", naming.UserAuthenticatorInstance, ahead.URL, corev1.PodRunning),
		instancePod("behind", naming.UserAuthenticatorInstance, behind.URL, corev1.PodRunning),
		// Neither a pod that has not started nor one that is going away is asked.
		instancePod("pending", naming.UserAuthenticatorInstance, "", corev1.PodPending),
		deleted,
		instancePod("system", naming.SystemAuthenticatorInstance, other.URL, corev1.PodRunning),
	)

	got, err := status.Loaded(t.Context(), naming.UserAuthenticatorInstance)
	if err != nil {
		t.Fatalf("Loaded: %v", err)
	}
	if got.Generation != 5 || len(got.Failed) != 2 {
		t.Errorf("status = %+v, want generation 5 and both refused records", got)
	}
	if !got.refused(broken) || !got.refused(refusedOther) || got.refused(served) || got.refused(nil) {
		t.Errorf("refused: %+v", got)
	}
	// The record of the same object with another cookie secret is another record: an object
	// created again under the name is not taken for the refused one.
	again := *refusedOther
	again.CookieSecret = served.CookieSecret
	if got.refused(&again) {
		t.Error("a record with another cookie secret is found by the identifier of the refused one")
	}
	// The value patch 018 of the image gives the same record: the two sides compute it apart.
	if id := statusID(served); id != "87a638d98469a6b98877b2d5dd0b19ae" {
		t.Errorf("statusID = %s, want the identifier the proxy computes", id)
	}

	// The answer is used for a while: the objects of an instance are reconciled in a row.
	if _, err := status.Loaded(t.Context(), naming.UserAuthenticatorInstance); err != nil || calls.Load() != 2 {
		t.Errorf("a second question made %d calls, error %v; want the first answer", calls.Load(), err)
	}
	status.now = func() time.Time { return time.Now().Add(statusTTL) }
	if _, err := status.Loaded(t.Context(), naming.UserAuthenticatorInstance); err != nil || calls.Load() != 4 {
		t.Errorf("a question after %v made %d calls, error %v; want the pods asked again", statusTTL, calls.Load(), err)
	}

	// A pod that is starting is asked like the others: its table is the one it will serve.
	starting := statusServer(t, &calls, http.StatusOK, `{"generation":3,"clients":1,"failed":[]}`)
	crashed := statusServer(t, &calls, http.StatusOK, `{}`)
	crashed.Close()
	status = newTestPodStatus(t,
		instancePod("ahead", naming.UserAuthenticatorInstance, ahead.URL, corev1.PodRunning),
		notReady(instancePod("starting", naming.UserAuthenticatorInstance, starting.URL, corev1.PodRunning)),
		// One that is not ready and does not answer gets no request and holds nobody.
		notReady(instancePod("crashed", naming.UserAuthenticatorInstance, crashed.URL, corev1.PodRunning)),
		instancePod("system", naming.SystemAuthenticatorInstance, other.URL, corev1.PodRunning),
	)
	if got, err := status.Loaded(t.Context(), naming.UserAuthenticatorInstance); err != nil || got.Generation != 3 {
		t.Errorf("with a pod that starts and one that crashed: %+v, %v; want generation 3", got, err)
	}

	system, err := status.Loaded(t.Context(), naming.SystemAuthenticatorInstance)
	if err != nil || system.Generation != 1 || len(system.Failed) != 0 {
		t.Errorf("the other instance: %+v, %v", system, err)
	}
}

func TestPodStatus_InstanceThatHasLoadedNothing(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	healthy := statusServer(t, &calls, http.StatusOK, `{"generation":7,"clients":3,"failed":[]}`)
	closed := statusServer(t, &calls, http.StatusOK, `{}`)
	closed.Close()

	tests := []struct {
		name string
		pods []client.Object
		want error
	}{
		{name: "no pod", want: errNoPods},
		{
			name: "no running pod",
			pods: []client.Object{instancePod("pending", naming.UserAuthenticatorInstance, "", corev1.PodPending)},
			want: errNoPods,
		},
		{
			name: "a pod that does not answer",
			pods: []client.Object{
				instancePod("healthy", naming.UserAuthenticatorInstance, healthy.URL, corev1.PodRunning),
				instancePod("closed", naming.UserAuthenticatorInstance, closed.URL, corev1.PodRunning),
			},
		},
		{
			// A pod that is not ready gets no request and is left out, but it is not the instance.
			name: "only a pod that is not ready and does not answer",
			pods: []client.Object{notReady(instancePod("starting", naming.UserAuthenticatorInstance, closed.URL, corev1.PodRunning))},
			want: errNoPods,
		},
		{
			name: "a pod that answers with an error",
			pods: []client.Object{instancePod("failing", naming.UserAuthenticatorInstance,
				statusServer(t, &calls, http.StatusInternalServerError, `{"generation":9}`).URL, corev1.PodRunning)},
		},
		{
			name: "a pod of an image without the table",
			pods: []client.Object{instancePod("old", naming.UserAuthenticatorInstance,
				statusServer(t, &calls, http.StatusOK, `<html>sign in</html>`).URL, corev1.PodRunning)},
		},
		{
			name: "a pod that sends elsewhere",
			pods: []client.Object{instancePod("redirecting", naming.UserAuthenticatorInstance,
				httptest.NewServer(http.RedirectHandler(healthy.URL+statusPath, http.StatusFound)).URL, corev1.PodRunning)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestPodStatus(t, tt.pods...).Loaded(t.Context(), naming.UserAuthenticatorInstance)
			if err == nil {
				t.Fatalf("status = %+v, want an error", got)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if got.Generation != 0 {
				t.Errorf("generation = %d with an error", got.Generation)
			}
		})
	}
}

func TestPodStatus_AddressOfAPod(t *testing.T) {
	t.Parallel()

	status, _ := NewPodStatus(newFakeClient(t)).(*podStatus)
	for ip, want := range map[string]string{
		"10.111.0.7": "https://10.111.0.7:8443/client-table",
		"fd00::7":    "https://[fd00::7]:8443/client-table",
	} {
		if got := status.url(ip); got != want {
			t.Errorf("address of %s = %s, want %s", ip, got, want)
		}
	}
}
