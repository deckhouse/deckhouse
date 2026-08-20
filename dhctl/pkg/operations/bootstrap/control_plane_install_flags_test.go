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

package bootstrap

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
)

// TestControlPlaneInstallFlagsFollowClusterConfiguration walks the derived flags all the way to the
// Deployment, because that is where they mean something: a nodeSelector on a role label no node
// carries, and an apiserver address taken from the pod's own host, both strand Deckhouse on a
// cluster dhctl did not build. The two environment variables are set by one block, so a case that
// checks a single one leaves half the condition open.
func TestControlPlaneInstallFlagsFollowClusterConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		metaConfig    *config.MetaConfig
		wantOwnedByUs bool
	}{
		{
			name:          "dhctl builds the control plane",
			metaConfig:    bootstrappedMetaConfig("Cloud"),
			wantOwnedByUs: true,
		},
		{
			name:          "the control plane already exists",
			metaConfig:    &config.MetaConfig{},
			wantOwnedByUs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installConfig := &config.DeckhouseInstaller{
				Registry: registry_mocks.ConfigBuilder(
					registry_mocks.WithModeUnmanaged(),
					registry_mocks.WithLegacyMode(),
				),
				Bundle:    "Minimal",
				LogLevel:  "Info",
				DevBranch: "pr1111",
			}

			setControlPlaneInstallFlags(installConfig, tt.metaConfig)

			deployment, err := deckhouse.CreateDeckhouseDeploymentManifest(installConfig)
			require.NoError(t, err)

			_, pinnedToControlPlane := deployment.Spec.Template.Spec.NodeSelector["node-role.kubernetes.io/control-plane"]
			assert.Equal(t, tt.wantOwnedByUs, pinnedToControlPlane, "control-plane nodeSelector")

			env := deployment.Spec.Template.Spec.Containers[0].Env
			assert.Equal(t, tt.wantOwnedByUs, hasEnvVar(env, "KUBERNETES_SERVICE_HOST"))
			assert.Equal(t, tt.wantOwnedByUs, hasEnvVar(env, "KUBERNETES_SERVICE_PORT"))
		})
	}
}

func hasEnvVar(env []apiv1.EnvVar, name string) bool {
	return slices.ContainsFunc(env, func(v apiv1.EnvVar) bool { return v.Name == name })
}
