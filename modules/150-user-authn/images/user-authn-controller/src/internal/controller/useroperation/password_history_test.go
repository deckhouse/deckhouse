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

package useroperation

import (
	"context"
	"encoding/base64"
	"net/http"
	"reflect"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

var resetTime = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func resetPasswordOperation() *unstructured.Unstructured {
	return userOperationObj("user-operation-01", resetTime, map[string]any{
		"type":          "ResetPassword",
		"initiatorType": "Admin",
		"user":          "admin",
		"resetPassword": map[string]any{"newPasswordHash": rawBcryptHash},
	})
}

// reset runs a ResetPassword of admin with the history limit and returns the Password after it.
func reset(t *testing.T, r *Reconciler, historyLimit uint64) *unstructured.Unstructured {
	t.Helper()
	r.passwordHistoryLimit = historyLimit
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", resetTime)
	got := getUnstructured(t, r, controller.PasswordGVK, naming.DexNamespace, passwordObjectName)
	hash, _, _ := unstructured.NestedString(got.Object, "hash")
	if hash != encodedBcryptHash {
		t.Errorf("hash = %q, want the reset hash", hash)
	}
	if requireReset, _, _ := unstructured.NestedBool(got.Object, "requireResetHashOnNextSuccLogin"); !requireReset {
		t.Error("requireResetHashOnNextSuccLogin = false, want true")
	}
	return got
}

func previousHashes(obj *unstructured.Unstructured) ([]any, bool) {
	history, found, _ := unstructured.NestedFieldCopy(obj.Object, "previousHashes")
	list, _ := history.([]any)
	return list, found
}

// The password from before a reset must stay refused at the forced change: Dex compares the new
// password with the hash, now the reset one, and with previousHashes.
func TestResetPassword_KeepsTheReplacedHashInTheHistory(t *testing.T) {
	t.Parallel()

	const oldPassword = "dh-test-secret-0123456789abcdef"
	oldHash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	encodedOld := base64.StdEncoding.EncodeToString(oldHash)
	pw := passwordObj("admin", map[string]any{"hash": encodedOld, "previousHashes": []any{"p1"}})
	r := newTestReconciler(t, resetTime, pw, resetPasswordOperation())

	got := reset(t, r, 5)

	history, _ := previousHashes(got)
	if want := []any{"p1", encodedOld}; !reflect.DeepEqual(history, want) {
		t.Fatalf("previousHashes = %v, want %v", history, want)
	}
	// What zone 220 of Dex does with the history at the forced change (Policy.CheckNew).
	last, err := base64.StdEncoding.DecodeString(history[len(history)-1].(string))
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(last, []byte(oldPassword)); err != nil {
		t.Errorf("the history does not refuse the password from before the reset: %v", err)
	}
}

func TestResetPassword_History(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		// setHistory writes previousHashes, history may be nil for a null.
		setHistory  bool
		history     []any
		limit       uint64
		want        []any
		wantPresent bool
	}{
		{name: "no history kept leaves none", limit: 0, wantPresent: false},
		{name: "no history kept leaves the history as is", setHistory: true, history: []any{"p1", "p2"}, limit: 0, want: []any{"p1", "p2"}, wantPresent: true},
		{name: "the first replaced hash starts the history", limit: 5, want: []any{"old"}, wantPresent: true},
		{name: "a null history starts too", setHistory: true, limit: 5, want: []any{"old"}, wantPresent: true},
		{name: "within the limit", setHistory: true, history: []any{"p1", "p2"}, limit: 5, want: []any{"p1", "p2", "old"}, wantPresent: true},
		{name: "trimmed to the limit", setHistory: true, history: []any{"p1", "p2", "p3"}, limit: 2, want: []any{"p3", "old"}, wantPresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			extra := map[string]any{"hash": "old"}
			if tc.setHistory {
				extra["previousHashes"] = tc.history
			}
			r := newTestReconciler(t, resetTime, passwordObj("admin", extra), resetPasswordOperation())

			got := reset(t, r, tc.limit)

			history, found := previousHashes(got)
			if found != tc.wantPresent || (tc.wantPresent && !reflect.DeepEqual(history, tc.want)) {
				t.Errorf("previousHashes = %v (present %v), want %v (present %v)", history, found, tc.want, tc.wantPresent)
			}
		})
	}
}

// An operation that runs again after its patch (requeued before its status was written) finds its
// own hash in place and must not push it into the history.
func TestResetPassword_RepeatedKeepsTheHistory(t *testing.T) {
	t.Parallel()

	pw := passwordObj("admin", map[string]any{"hash": encodedBcryptHash, "previousHashes": []any{"p1", "old"}})
	r := newTestReconciler(t, resetTime, pw, resetPasswordOperation())

	got := reset(t, r, 5)

	history, _ := previousHashes(got)
	if want := []any{"p1", "old"}; !reflect.DeepEqual(history, want) {
		t.Errorf("previousHashes = %v, want %v", history, want)
	}
}

// A password change by Dex between the read and the patch fails the test of the patch; the reset
// runs again on what Dex wrote and keeps it in the history.
func TestResetPassword_RetriesOnAConcurrentChange(t *testing.T) {
	t.Parallel()

	patches := 0
	funcs := interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if obj.GetObjectKind().GroupVersionKind().Kind != controller.PasswordGVK.Kind || patch.Type() != types.JSONPatchType {
				return c.Patch(ctx, obj, patch, opts...)
			}
			patches++
			if patches == 1 {
				changed := controller.Object(controller.PasswordGVK)
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), changed); err != nil {
					return err
				}
				changed.Object["hash"] = "changed"
				changed.Object["previousHashes"] = []any{"p1", "old"}
				if err := c.Update(ctx, changed); err != nil {
					return err
				}
			}
			// The fake client returns the error of the JSON Patch as is; the API server answers 422.
			if err := c.Patch(ctx, obj, patch, opts...); err != nil {
				if _, isStatus := err.(apierrors.APIStatus); isStatus {
					return err
				}
				return apierrors.NewGenericServerResponse(http.StatusUnprocessableEntity, "", schema.GroupResource{}, "", err.Error(), 0, false)
			}
			return nil
		},
	}
	pw := passwordObj("admin", map[string]any{"hash": "old", "previousHashes": []any{"p1"}})
	r := newTestReconcilerWithFuncs(t, resetTime, funcs, pw, resetPasswordOperation())

	got := reset(t, r, 5)

	if patches != 2 {
		t.Errorf("Password patched %d times, want 2 (a failed test, then the retry)", patches)
	}
	history, _ := previousHashes(got)
	if want := []any{"p1", "old", "changed"}; !reflect.DeepEqual(history, want) {
		t.Errorf("previousHashes = %v, want %v", history, want)
	}
}
