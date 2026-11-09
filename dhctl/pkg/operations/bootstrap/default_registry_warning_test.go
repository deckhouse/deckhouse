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
	"bytes"
	"log/slog"
	"testing"

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// PrepareDeckhouseInstallConfig no longer says the default-registry warning itself, because dhctl
// deckhouse create-deployment shares it. The bootstrap has to say it, and this is what fails if the
// call drops out of the path bootstrap and bootstrap-phase install-deckhouse take.
func TestBootstrapInstallConfigAnnouncesDefaultRegistry(t *testing.T) {
	const clusterConfig = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`
	const registryModuleConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  enabled: true
  version: 1
  settings:
    mode: Managed
    primary:
      upstream:
        scheme: HTTPS
        host: registry.deckhouse.io
        path: /deckhouse/ce
`
	const warning = "No registry settings were given: installing Deckhouse CE from registry.deckhouse.io/deckhouse/ce"

	tests := []struct {
		name     string
		config   string
		wantWarn bool
	}{
		{name: "no registry settings", config: clusterConfig, wantWarn: true},
		{name: "a registry ModuleConfig written by the user", config: clusterConfig + registryModuleConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig, err := config.ParseConfigFromData(t.Context(), tt.config, config.DummyValidatorProvider(), nil)
			require.NoError(t, err)

			var logged bytes.Buffer
			ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

			_, err = prepareBootstrapInstallConfig(ctx, metaConfig, &options.New().Global)
			require.NoError(t, err)

			if tt.wantWarn {
				require.Contains(t, logged.String(), warning)
			} else {
				require.NotContains(t, logged.String(), warning)
			}
		})
	}
}
