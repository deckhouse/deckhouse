// Copyright 2026 Flant JSC
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

package kubeerrors

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// impersonationDenied is what the kubectl proxy relays while the binding that lets
// kubernetes-admin impersonate is unavailable — an apiserver restarting, or
// control-plane-manager re-creating the kubeadm:cluster-admins ClusterRoleBinding.
const impersonationDenied = `users "dhctl" is forbidden: User "kubernetes-admin" cannot impersonate resource "users" in API group "" at the cluster scope`

func statusErr(code int32, reason metav1.StatusReason, message string) error {
	return &apierrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    code,
		Reason:  reason,
		Message: message,
	}}
}

func forbiddenErr(message string) error {
	return statusErr(403, metav1.StatusReasonForbidden, message)
}

func unauthorizedErr(message string) error {
	return statusErr(401, metav1.StatusReasonUnauthorized, message)
}

func TestIsPermanentAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		// want is indexed by auth mode: the same error is transient over the impersonating
		// kube-proxy and fatal when dhctl runs as credentials it was handed.
		wantKubeProxy      bool
		wantOwnCredentials bool
	}{
		{
			name:               "nil error",
			err:                nil,
			wantKubeProxy:      false,
			wantOwnCredentials: false,
		},
		{
			name:               "not an auth error",
			err:                apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, "master-0"),
			wantKubeProxy:      false,
			wantOwnCredentials: false,
		},
		{
			name:               "transport error",
			err:                errors.New("dial tcp 127.0.0.1:6445: connect: connection refused"),
			wantKubeProxy:      false,
			wantOwnCredentials: false,
		},
		{
			name:               "impersonation denied",
			err:                forbiddenErr(impersonationDenied),
			wantKubeProxy:      false,
			wantOwnCredentials: true,
		},
		{
			name:               "impersonation denied, wrapped by the caller",
			err:                fmt.Errorf("get nodes count: %w", forbiddenErr(impersonationDenied)),
			wantKubeProxy:      false,
			wantOwnCredentials: true,
		},
		{
			name:               "RBAC denial",
			err:                forbiddenErr(`nodes is forbidden: User "limited" cannot list resource "nodes" in API group "" at the cluster scope`),
			wantKubeProxy:      false,
			wantOwnCredentials: true,
		},
		{
			name:               "unauthorized",
			err:                unauthorizedErr("Unauthorized"),
			wantKubeProxy:      false,
			wantOwnCredentials: true,
		},
		{
			name: "denial the apiserver asks us to retry",
			err: &apierrors.StatusError{ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    403,
				Reason:  metav1.StatusReasonForbidden,
				Message: "nodes is forbidden: authorizer is not ready",
				Details: &metav1.StatusDetails{RetryAfterSeconds: 1},
			}},
			wantKubeProxy:      false,
			wantOwnCredentials: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyCtx := WithAuthMode(t.Context(), AuthModeKubeProxy)
			require.Equal(t, tt.wantKubeProxy, IsPermanentAuthError(proxyCtx, tt.err), "over kube-proxy")

			ownCtx := WithAuthMode(t.Context(), AuthModeOwnCredentials)
			require.Equal(t, tt.wantOwnCredentials, IsPermanentAuthError(ownCtx, tt.err), "with own credentials")
		})
	}
}

// A context that was never stamped must not turn a transient denial into a failed operation, so
// the unset mode behaves like the SSH tunnel — which is also dhctl's default way to connect.
func TestIsPermanentAuthErrorDefaultsToRetriable(t *testing.T) {
	require.Equal(t, AuthModeKubeProxy, AuthModeFromContext(context.Background()))
	require.False(t, IsPermanentAuthError(context.Background(), forbiddenErr(impersonationDenied)))
	require.False(t, IsPermanentAuthError(context.Background(), unauthorizedErr("Unauthorized")))
}

func TestAuthModeFromContext(t *testing.T) {
	require.Equal(t, AuthModeOwnCredentials,
		AuthModeFromContext(WithAuthMode(t.Context(), AuthModeOwnCredentials)))

	// An inner loop may re-stamp the mode; the innermost value wins.
	ctx := WithAuthMode(WithAuthMode(t.Context(), AuthModeOwnCredentials), AuthModeKubeProxy)
	require.Equal(t, AuthModeKubeProxy, AuthModeFromContext(ctx))
}
