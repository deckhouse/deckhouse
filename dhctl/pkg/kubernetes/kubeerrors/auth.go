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

// Package kubeerrors classifies Kubernetes API errors for dhctl retry loops.
package kubeerrors

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// AuthMode describes how dhctl's requests reach the apiserver, which is what decides whether an
// authentication or authorization failure can clear up on its own. It is resolved once, from the
// kube settings the connection was built with, and carried on the kube client.
type AuthMode int

const (
	// AuthModeKubeProxy is dhctl's default: no kube flags were given, so requests go through an
	// SSH tunnel to a kubectl proxy started on a master node with
	//
	//	kubectl proxy --as=dhctl --as-group=system:masters --kubeconfig /etc/kubernetes/admin.conf
	//
	// system:masters is the apiserver's hardcoded superuser group, so no authorization decision
	// can ever deny dhctl in this mode. A 401/403 therefore never means "you may not do this" —
	// it means the apiserver is not currently in a state to answer, e.g.:
	//
	//   - an instance is restarting (converge, control-plane update, node reboot) and its
	//     authenticators or RBAC informers are not warm yet;
	//   - control-plane-manager is re-creating the kubeadm:cluster-admins ClusterRoleBinding — a
	//     roleRef change is an immutable-field update, i.e. a Delete+Create, so kubernetes-admin
	//     genuinely holds no permissions for a moment, and the proxy's impersonation is refused:
	//
	//     users "dhctl" is forbidden: User "kubernetes-admin" cannot impersonate
	//     resource "users" in API group "" at the cluster scope
	//
	//   - the authorization webhook (user-authz) is unreachable, so nothing can be evaluated.
	//
	// All of those pass, so retrying is the correct response. This is the zero value: a client
	// whose mode was never resolved is treated as retriable rather than failing an operation on
	// an error that was probably transient.
	AuthModeKubeProxy AuthMode = iota

	// AuthModeOwnCredentials means requests carry credentials dhctl was handed rather than the
	// master's admin.conf: --kubeconfig, --kube-client-from-cluster, a programmatic rest config,
	// or a local run using the ambient kubeconfig. There is no impersonation and no guarantee of
	// cluster-admin, so a 401/403 is a verdict about those credentials and every retry gets the
	// same answer. Loops must stop and report it instead of spending their attempt budget.
	AuthModeOwnCredentials
)

func (m AuthMode) String() string {
	switch m {
	case AuthModeKubeProxy:
		return "kube-proxy-over-ssh"
	case AuthModeOwnCredentials:
		return "own-credentials"
	default:
		return "unknown"
	}
}

type authModeKey struct{}

// WithAuthMode returns a context carrying the mode its retry loops classify auth failures with.
// It is stamped once per invocation, on the context every operation runs under — see
// providerinitializer.WithKubeAuthMode — so no retry loop has to plumb it itself.
func WithAuthMode(ctx context.Context, mode AuthMode) context.Context {
	return context.WithValue(ctx, authModeKey{}, mode)
}

// AuthModeFromContext returns the mode WithAuthMode put in ctx, or AuthModeKubeProxy when the
// context carries none.
func AuthModeFromContext(ctx context.Context) AuthMode {
	mode, ok := ctx.Value(authModeKey{}).(AuthMode)
	if !ok {
		return AuthModeKubeProxy
	}

	return mode
}

// IsPermanentAuthError reports whether err is an authentication/authorization failure that a
// retry loop should stop on instead of spending its whole attempt budget on an answer that will
// not change. That depends entirely on how the connection was built — see AuthMode.
//
// Note that an admission-webhook rejection does not reach this decision: the apiserver reports
// those with code 400 unless the webhook explicitly sets 403, so they surface as
// BadRequest/Invalid, which the call sites classify on their own.
func IsPermanentAuthError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}

	if !apierrors.IsForbidden(err) && !apierrors.IsUnauthorized(err) {
		return false
	}

	// The apiserver asked us to come back later, so it does not consider the denial final.
	if _, ok := apierrors.SuggestsClientDelay(err); ok {
		return false
	}

	return AuthModeFromContext(ctx) == AuthModeOwnCredentials
}
