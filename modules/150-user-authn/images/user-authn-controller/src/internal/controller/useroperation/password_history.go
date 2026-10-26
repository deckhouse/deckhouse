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
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
)

// jsonPatchOp is one operation of a JSON Patch (RFC 6902).
type jsonPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// resetPasswordHash gives the Password name the hash (base64 of a bcrypt hash, as Dex stores it)
// and makes the user change it at the next sign-in.
//
// When the password policy keeps a history, the replaced hash joins previousHashes, trimmed to the
// last passwordHistoryLimit, as Dex does when the user changes the password. Otherwise the forced
// change after a reset compares the new password with the reset one and a history without the
// password from before the reset, and the user can set it again.
//
// The patch tests the hash and the history it was built from: a change of the password that
// lands between the read and the write fails the patch (422 from the API server), and the
// read-build-write runs again on the new values instead of dropping the hash written meanwhile.
// The Password is read past the cache, which may not have the change yet.
func (r *Reconciler) resetPasswordHash(ctx context.Context, namespace, name, hash string) error {
	return retry.OnError(retry.DefaultRetry, isPatchRace, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		obj := controller.Object(controller.PasswordGVK)
		if err := r.apiReader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			return fmt.Errorf("get password %s: %w", name, err)
		}
		ops, err := resetHashPatch(obj, hash, r.passwordHistoryLimit)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(ops)
		if err != nil {
			return fmt.Errorf("marshal json patch for password %s: %w", name, err)
		}
		if err := r.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, raw)); err != nil {
			return fmt.Errorf("patch password %s: %w", name, err)
		}
		return nil
	})
}

// isPatchRace reports a patch refused because the object changed since it was read: the API
// server answers a failed test of a JSON Patch with 422 Invalid.
func isPatchRace(err error) bool {
	return apierrors.IsInvalid(err) || apierrors.IsConflict(err)
}

// resetHashPatch builds the JSON Patch of resetPasswordHash from the Password as it was read.
// historyLimit 0 leaves previousHashes as it is, as Dex does without a history.
func resetHashPatch(obj *unstructured.Unstructured, hash string, historyLimit uint64) ([]jsonPatchOp, error) {
	current, hasHash, err := unstructured.NestedString(obj.Object, "hash")
	if err != nil {
		return nil, fmt.Errorf("password %s hash: %w", obj.GetName(), err)
	}
	if hasHash && current == hash {
		// The operation ran again after its patch (a requeue): the hash is its own, and the hash it
		// replaced is in the history already.
		return []jsonPatchOp{{Op: "add", Path: "/requireResetHashOnNextSuccLogin", Value: true}}, nil
	}
	previous, hasHistory, err := unstructured.NestedFieldCopy(obj.Object, "previousHashes")
	if err != nil {
		return nil, fmt.Errorf("password %s previousHashes: %w", obj.GetName(), err)
	}

	var ops []jsonPatchOp
	if hasHash {
		ops = append(ops, jsonPatchOp{Op: "test", Path: "/hash", Value: current})
	}
	if historyLimit > 0 && hasHash {
		var history []any
		if hasHistory {
			// Dex writes previousHashes with the hash, so the test of the hash alone would do; the
			// test of the history keeps the patch from replacing one written by anything else.
			ops = append(ops, jsonPatchOp{Op: "test", Path: "/previousHashes", Value: previous})
			if previous != nil {
				list, ok := previous.([]any)
				if !ok {
					return nil, fmt.Errorf("password %s previousHashes is %T, not a list", obj.GetName(), previous)
				}
				history = list
			}
		}
		history = append(history, current)
		if n := uint64(len(history)); n > historyLimit {
			history = history[n-historyLimit:]
		}
		// add replaces an existing member of an object, so it works with and without the history.
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/previousHashes", Value: history})
	}
	return append(ops,
		jsonPatchOp{Op: "add", Path: "/hash", Value: hash},
		jsonPatchOp{Op: "add", Path: "/requireResetHashOnNextSuccLogin", Value: true},
	), nil
}
