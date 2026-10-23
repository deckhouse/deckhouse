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

package useridentity

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// Outcome tells what Mutate did.
type Outcome int

const (
	// OutcomeUnknown is the zero value; Mutate never returns it without an error.
	OutcomeUnknown Outcome = iota
	// OutcomeUnchanged means nothing was written.
	OutcomeUnchanged
	// OutcomeCreated means the object was created.
	OutcomeCreated
	// OutcomeUpdated means the object was updated.
	OutcomeUpdated
)

// MutateFunc gets the object as it is now, or nil when it does not exist, and returns the object
// to write, or nil to write nothing. It may change and return current. It runs again on every
// retry, after a fresh read, so whatever it reads to decide must be read inside it.
type MutateFunc func(current *Identity) (*Identity, error)

// Mutate reads the UserIdentity name through reader, which must bypass the cache, hands it to fn,
// and writes the result: Create when the object did not exist, Update with the resourceVersion
// that was read otherwise. A conflict or an AlreadyExists means somebody else wrote in between,
// and the whole read-decide-write runs again.
//
// Reading the UserIdentity before whatever fn reads is what makes a stale decision fail: any
// write to the object after this read, by Dex or by another worker, changes its resourceVersion.
func Mutate(ctx context.Context, reader client.Reader, writer client.Writer, name string, fn MutateFunc) (Outcome, error) {
	outcome := OutcomeUnknown
	err := retry.OnError(retry.DefaultRetry, isWriteRace, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		outcome, err = mutateOnce(ctx, reader, writer, name, fn)
		return err
	})
	if err != nil {
		return OutcomeUnknown, err
	}
	return outcome, nil
}

func mutateOnce(ctx context.Context, reader client.Reader, writer client.Writer, name string, fn MutateFunc) (Outcome, error) {
	obj := controller.Object(controller.UserIdentityGVK)
	err := reader.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	var current *Identity
	switch {
	case err == nil:
		current = FromUnstructured(obj)
	case apierrors.IsNotFound(err):
	default:
		return OutcomeUnknown, fmt.Errorf("get useridentity %s: %w", name, err)
	}

	var before map[string]any
	if current != nil {
		before = obj.DeepCopy().Object
	}

	desired, err := fn(current)
	if err != nil {
		return OutcomeUnknown, err
	}
	if desired == nil {
		return OutcomeUnchanged, nil
	}
	if desired.Object().GetName() != name {
		return OutcomeUnknown, fmt.Errorf("useridentity %s: mutation returned object %s", name, desired.Object().GetName())
	}

	if current == nil {
		if err := writer.Create(ctx, desired.Object()); err != nil {
			return OutcomeUnknown, fmt.Errorf("create useridentity %s: %w", name, err)
		}
		return OutcomeCreated, nil
	}
	if reflect.DeepEqual(before, desired.Object().Object) {
		return OutcomeUnchanged, nil
	}
	if err := writer.Update(ctx, desired.Object()); err != nil {
		return OutcomeUnknown, fmt.Errorf("update useridentity %s: %w", name, err)
	}
	return OutcomeUpdated, nil
}

func isWriteRace(err error) bool {
	return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
}

// Delete deletes the UserIdentity name; a missing object is not an error.
func Delete(ctx context.Context, writer client.Writer, name string) error {
	obj := controller.Object(controller.UserIdentityGVK)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	if err := writer.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete useridentity %s: %w", name, err)
	}
	return nil
}
