/*
Copyright 2023 Flant JSC

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

package checker

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"d8.io/upmeter/pkg/check"
)

// KubeObjectBasicLifecycle checks the creation and deletion of an object in
// kube-apiserver. Hence, all errors in kube-apiserver calls result in probe
// fails.
type KubeObjectBasicLifecycle struct {
	preflight Doer
	getter    Doer
	creator   Doer
	deleter   Doer
}

func (c *KubeObjectBasicLifecycle) Check() check.Error {
	ctx := context.TODO()
	if err := c.preflight.Do(ctx); err != nil {
		return check.ErrUnknown("preflight: %v", err)
	}

	// Check garbage
	if getErr := c.getter.Do(ctx); getErr != nil && !apierrors.IsNotFound(getErr) {
		// Unexpected error
		return check.ErrFail("getting garbage: %v", getErr)
	} else if getErr == nil {
		// Garbage object exists, cleaning it and skipping this run.
		if delErr := c.deleter.Do(ctx); delErr != nil {
			return check.ErrFail("deleting garbage: %v", delErr)
		}
		return check.ErrUnknown("cleaned garbage")
	}

	// The actual check
	if createErr := c.creator.Do(ctx); createErr != nil {
		// Unexpected error
		return check.ErrFail("creating: %v", createErr)
	}
	if delErr := c.deleter.Do(ctx); delErr != nil {
		// Unexpected error
		return check.ErrFail("deleting: %v", delErr)
	}

	return nil
}
