// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/kube"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

func TestKubeAuthMode(t *testing.T) {
	tests := []struct {
		name     string
		kubeOpts *options.KubeOptions
		want     kubeerrors.AuthMode
	}{
		{
			// dhctl's default: lib-connection tunnels to a kubectl proxy on a master that
			// impersonates system:masters, so no denial there is final.
			name:     "no kube flags",
			kubeOpts: &options.KubeOptions{},
			want:     kubeerrors.AuthModeKubeProxy,
		},
		{
			name:     "nil options",
			kubeOpts: nil,
			want:     kubeerrors.AuthModeKubeProxy,
		},
		{
			name:     "kubeconfig",
			kubeOpts: &options.KubeOptions{Config: "/root/.kube/config"},
			want:     kubeerrors.AuthModeOwnCredentials,
		},
		{
			// A context without a kubeconfig names no credentials: lib-connection's switch only
			// reads it when KubeConfig is set, so the connection is still the SSH proxy.
			name:     "kubeconfig context alone",
			kubeOpts: &options.KubeOptions{ConfigContext: "prod"},
			want:     kubeerrors.AuthModeKubeProxy,
		},
		{
			name:     "kubeconfig with context",
			kubeOpts: &options.KubeOptions{Config: "/root/.kube/config", ConfigContext: "prod"},
			want:     kubeerrors.AuthModeOwnCredentials,
		},
		{
			name:     "in-cluster service account",
			kubeOpts: &options.KubeOptions{InCluster: true},
			want:     kubeerrors.AuthModeOwnCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, KubeAuthMode(tt.kubeOpts))

			// The mode must follow lib-connection's own predicate, or dhctl would classify
			// errors for a connection it does not actually have.
			if tt.kubeOpts != nil {
				cfg := &kube.Config{
					KubeConfig:          tt.kubeOpts.Config,
					KubeConfigContext:   tt.kubeOpts.ConfigContext,
					KubeConfigInCluster: tt.kubeOpts.InCluster,
				}
				require.Equal(t, cfg.OverSSH(), tt.want == kubeerrors.AuthModeKubeProxy)
			}

			// Same answer, stamped on a context for the retry loops to read.
			ctx := WithKubeAuthMode(t.Context(), tt.kubeOpts)
			require.Equal(t, tt.want, kubeerrors.AuthModeFromContext(ctx))
		})
	}
}

// KubeOptions.IsDefined answers a different question — "were any kube flags given" — and the two
// diverge on exactly one input. Pinned here so nobody swaps one for the other.
func TestKubeAuthModeIsNotIsDefined(t *testing.T) {
	contextOnly := &options.KubeOptions{ConfigContext: "prod"}

	require.True(t, contextOnly.IsDefined())
	require.Equal(t, kubeerrors.AuthModeKubeProxy, KubeAuthMode(contextOnly),
		"a context without a kubeconfig does not change how we authenticate")

	for _, kubeOpts := range []*options.KubeOptions{
		{},
		{Config: "/root/.kube/config"},
		{InCluster: true},
	} {
		require.Equal(t, kubeOpts.IsDefined(), KubeAuthMode(kubeOpts) == kubeerrors.AuthModeOwnCredentials)
	}
}
