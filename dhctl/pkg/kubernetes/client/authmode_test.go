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
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

func TestAuthModeForInitParams(t *testing.T) {
	tests := []struct {
		name       string
		params     *KubernetesInitParams
		isLocalRun bool
		want       kubeerrors.AuthMode
	}{
		{
			name:   "no kube settings: ssh tunnel to the impersonating kubectl proxy",
			params: &KubernetesInitParams{},
			want:   kubeerrors.AuthModeKubeProxy,
		},
		{
			name:   "kubeconfig",
			params: &KubernetesInitParams{KubeConfig: "/root/.kube/config"},
			want:   kubeerrors.AuthModeOwnCredentials,
		},
		{
			name:   "in-cluster service account",
			params: &KubernetesInitParams{KubeConfigInCluster: true},
			want:   kubeerrors.AuthModeOwnCredentials,
		},
		{
			name:   "programmatic rest config",
			params: &KubernetesInitParams{RestConfig: &rest.Config{Host: "https://127.0.0.1:6443"}},
			want:   kubeerrors.AuthModeOwnCredentials,
		},
		{
			// lib-connection only starts the impersonating proxy for an SSH node interface, so a
			// local run talks to the apiserver with the ambient credentials.
			name:       "local run",
			params:     &KubernetesInitParams{},
			isLocalRun: true,
			want:       kubeerrors.AuthModeOwnCredentials,
		},
		{
			name:   "nil params",
			params: nil,
			want:   kubeerrors.AuthModeKubeProxy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, AuthModeForInitParams(tt.params, tt.isLocalRun))
		})
	}
}

type providerStub struct {
	libcon.KubeProvider
}

func TestAuthModeOfProvider(t *testing.T) {
	t.Run("declared by the provider", func(t *testing.T) {
		p := NewAuthModeAwareProvider(&providerStub{}, kubeerrors.AuthModeOwnCredentials)
		require.Equal(t, kubeerrors.AuthModeOwnCredentials, AuthModeOfProvider(p))
	})

	t.Run("provider that declares nothing stays retriable", func(t *testing.T) {
		require.Equal(t, kubeerrors.AuthModeKubeProxy, AuthModeOfProvider(&providerStub{}))
	})
}

func TestFromProviderCarriesAuthMode(t *testing.T) {
	p := NewAuthModeAwareProvider(&providerStub{}, kubeerrors.AuthModeOwnCredentials)

	kubeCl := FromProvider(p, nil)
	require.Equal(t, kubeerrors.AuthModeOwnCredentials, kubeCl.AuthMode)
	require.Equal(t, kubeerrors.AuthModeOwnCredentials,
		kubeerrors.AuthModeFromContext(kubeCl.AuthModeCtx(t.Context())))
}

func TestInitContextResolvesAuthMode(t *testing.T) {
	kubeCl := NewKubernetesClient()

	err := kubeCl.InitContext(t.Context(), &KubernetesInitParams{
		RestConfig: &rest.Config{Host: "https://127.0.0.1:6443"},
	})
	require.NoError(t, err)
	require.Equal(t, kubeerrors.AuthModeOwnCredentials, kubeCl.AuthMode)
}
