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

package client

import (
	"context"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

// AuthModeAwareProvider is a kube provider that knows how the clients it hands out authenticate.
// providerinitializer resolves the mode from the kube settings the provider was built with and
// wraps every provider it returns, so the answer travels with the provider instead of being
// re-derived (or guessed) by each operation.
type AuthModeAwareProvider interface {
	libcon.KubeProvider

	AuthMode() kubeerrors.AuthMode
}

// NewAuthModeAwareProvider decorates p so its clients can be built with FromProvider.
func NewAuthModeAwareProvider(p libcon.KubeProvider, mode kubeerrors.AuthMode) AuthModeAwareProvider {
	return &authModeAwareProvider{KubeProvider: p, mode: mode}
}

type authModeAwareProvider struct {
	libcon.KubeProvider

	mode kubeerrors.AuthMode
}

func (p *authModeAwareProvider) AuthMode() kubeerrors.AuthMode {
	return p.mode
}

// AuthModeOfProvider returns the auth mode p builds its clients with. Providers that do not
// declare one (test fakes, providers built outside providerinitializer) report the zero value,
// kubeerrors.AuthModeKubeProxy, which keeps auth failures retriable.
func AuthModeOfProvider(p libcon.KubeProvider) kubeerrors.AuthMode {
	if aware, ok := p.(AuthModeAwareProvider); ok {
		return aware.AuthMode()
	}

	return kubeerrors.AuthModeKubeProxy
}

// FromProvider wraps a client obtained from p into a dhctl KubernetesClient, carrying p's auth
// mode so retry loops can tell a restarting apiserver from a credentials problem.
func FromProvider(p libcon.KubeProvider, kubeCl libcon.KubeClient) *KubernetesClient {
	return &KubernetesClient{
		KubeClient: kubeCl,
		AuthMode:   AuthModeOfProvider(p),
	}
}

// AuthModeCtx returns ctx carrying this client's auth mode, for the retry loops that classify its
// errors with kubeerrors.IsPermanentAuthError.
func (k *KubernetesClient) AuthModeCtx(ctx context.Context) context.Context {
	return kubeerrors.WithAuthMode(ctx, k.AuthMode)
}

// AuthModeForInitParams resolves the auth mode of a connection described by params: anything that
// names its own credentials means AuthModeOwnCredentials, and so does a local run, which uses the
// ambient kubeconfig instead of an impersonating kubectl proxy on a master (lib-connection only
// starts that proxy for an SSH node interface). Everything else is the default SSH tunnel.
func AuthModeForInitParams(params *KubernetesInitParams, isLocalRun bool) kubeerrors.AuthMode {
	if params == nil {
		return kubeerrors.AuthModeKubeProxy
	}

	if params.KubeConfigInCluster || params.KubeConfig != "" || params.RestConfig != nil || isLocalRun {
		return kubeerrors.AuthModeOwnCredentials
	}

	return kubeerrors.AuthModeKubeProxy
}
