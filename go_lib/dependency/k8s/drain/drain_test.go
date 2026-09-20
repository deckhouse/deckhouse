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

package drain

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsTransientEvictionError(t *testing.T) {
	// The exact shape an unreachable admission webhook takes at the eviction call site.
	webhookUnreachable := apierrors.NewInternalError(errors.New(
		`failed calling webhook "virt-launcher-eviction-interceptor.kubevirt.io": Post "https://virt-api:24192/pod-eviction-validate": EOF`))

	budgetDenied := apierrors.NewTooManyRequests("Cannot evict pod as it would violate the pod's disruption budget", 5)

	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Resource: "pods"}, "some-pod", errors.New("not allowed"))

	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "some-pod")

	serverTimeout := apierrors.NewServerTimeout(schema.GroupResource{Resource: "pods"}, "create", 1)

	unavailable := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status: metav1.StatusFailure,
		Code:   503,
		Reason: metav1.StatusReasonServiceUnavailable,
	}}

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"unreachable webhook is transient", webhookUnreachable, true},
		{"server timeout is transient", serverTimeout, true},
		{"service unavailable is transient", unavailable, true},
		{"disruption budget is not transient", budgetDenied, false},
		{"forbidden is not transient", forbidden, false},
		{"not found is not transient", notFound, false},
	} {
		if got := isTransientEvictionError(tc.err); got != tc.want {
			t.Errorf("%s: isTransientEvictionError() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
