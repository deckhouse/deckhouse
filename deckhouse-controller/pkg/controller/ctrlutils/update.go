// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ctrlutils

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MutateFn func() error

type UpdateOptions struct {
	OnErrorBackoff         wait.Backoff
	RetryOnConflictBackoff wait.Backoff

	statusUpdate bool
}

func (o *UpdateOptions) WithOnErrorBackoff(b *wait.Backoff) {
	if b == nil {
		return
	}

	o.OnErrorBackoff = *b
}

func (o *UpdateOptions) WithRetryOnConflictBackoff(b *wait.Backoff) {
	if b == nil {
		return
	}

	o.RetryOnConflictBackoff = *b
}

func (o *UpdateOptions) withStatusUpdate() {
	o.statusUpdate = true
}

var ErrCanNotMutateNameOrNamespace = errors.New("MutateFn cannot mutate object name and/or object namespace")

func UpdateWithRetry(ctx context.Context, c client.Client, obj client.Object, f MutateFn, opts ...UpdateOption) error {
	options := &UpdateOptions{
		OnErrorBackoff:         retry.DefaultRetry,
		RetryOnConflictBackoff: retry.DefaultRetry,
	}

	for _, fn := range opts {
		fn.Apply(options)
	}

	key := client.ObjectKeyFromObject(obj)

	return retry.OnError(options.OnErrorBackoff, apierrors.IsServiceUnavailable, func() error {
		return retry.RetryOnConflict(options.RetryOnConflictBackoff, func() error {
			if err := c.Get(ctx, key, obj); err != nil {
				return client.IgnoreNotFound(err)
			}

			existing := obj.DeepCopyObject()
			if err := mutate(f, key, obj); err != nil {
				return err
			}

			if equality.Semantic.DeepEqual(existing, obj) {
				return nil
			}

			if options.statusUpdate {
				err := c.Status().Update(ctx, obj)
				if err != nil {
					return fmt.Errorf("status update: %w", err)
				}

				return nil
			}

			err := c.Update(ctx, obj)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}

			return nil
		})
	})
}

func UpdateStatusWithRetry(ctx context.Context, c client.Client, obj client.Object, f MutateFn, opts ...UpdateOption) error {
	opts = append(opts, withStatusUpdate())

	return UpdateWithRetry(ctx, c, obj, f, opts...)
}

// mutate wraps a MutateFn and applies validation to its result.
func mutate(f MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}

	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return ErrCanNotMutateNameOrNamespace
	}

	return nil
}
