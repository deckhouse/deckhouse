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

package providerinitializer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/lib-connection/pkg/kube"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

// The mapping must follow kube.Config.OverSSH(), which is what lib-connection itself uses to
// decide whether to tunnel to the impersonating kubectl proxy on a master.
func TestAuthModeForKubeConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *kube.Config
		want kubeerrors.AuthMode
	}{
		{
			name: "no kube settings: over ssh",
			cfg:  &kube.Config{},
			want: kubeerrors.AuthModeKubeProxy,
		},
		{
			name: "kubeconfig",
			cfg:  &kube.Config{KubeConfig: "/root/.kube/config"},
			want: kubeerrors.AuthModeOwnCredentials,
		},
		{
			name: "in-cluster",
			cfg:  &kube.Config{KubeConfigInCluster: true},
			want: kubeerrors.AuthModeOwnCredentials,
		},
		{
			name: "local kube client",
			cfg:  &kube.Config{LocalKubeClient: true},
			want: kubeerrors.AuthModeOwnCredentials,
		},
		{
			name: "rest config",
			cfg:  &kube.Config{RestConfig: &rest.Config{Host: "https://127.0.0.1:6443"}},
			want: kubeerrors.AuthModeOwnCredentials,
		},
		{
			name: "nil config",
			cfg:  nil,
			want: kubeerrors.AuthModeOwnCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, authModeForKubeConfig(tt.cfg))

			if tt.cfg != nil {
				// Guard against the two drifting apart: OverSSH is the source of truth.
				require.Equal(t, tt.cfg.OverSSH(), tt.want == kubeerrors.AuthModeKubeProxy)
			}
		})
	}
}
