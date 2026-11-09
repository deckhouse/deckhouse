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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const previousModesStaticCluster = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.33"
clusterDomain: "cluster.local"
`

func previousModesDeckhouseMC(settings string) string {
	return `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
` + settings
}

var (
	previousModesWithoutRegistry = previousModesDeckhouseMC("    bundle: Default\n")
	previousModesProxy           = previousModesDeckhouseMC(
		"    registry:\n      mode: Proxy\n      proxy:\n        imagesRepo: r.example.com/test\n        scheme: HTTPS\n",
	)
	previousModesLocal = previousModesDeckhouseMC("    registry:\n      mode: Local\n")
)

func writePreviousModesFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// finalizingPEC is preparationPEC with the deferred Finalize that Bootstrap always calls.
type finalizingPEC struct {
	preparationPEC
}

func (p *finalizingPEC) Finalize(_ context.Context, _ state.Cache) error { return nil }

// TestBootstrapRefusesThePreviousRegistryImplementationInTheResources drives dhctl bootstrap with
// a deckhouse ModuleConfig that names Proxy or Local in the resources of the installation: from
// --resources, and in the order the server's Bootstrap writes commander's documents (configuration,
// init resources, resources).
//
// The refusal has to come from Preparation, before the walk announces anything else: base
// infrastructure, the first master and every later phase are never reached. Before, only the copy
// the installation read was checked, and a copy that was neither the first nor the last reached
// the cluster and was refused there, with the infrastructure already created.
func TestBootstrapRefusesThePreviousRegistryImplementationInTheResources(t *testing.T) {
	t.Parallel()

	for mode, refused := range map[string]string{"Proxy": previousModesProxy, "Local": previousModesLocal} {
		cases := map[string]struct {
			configs   []string
			resources string
		}{
			"--resources, the only copy": {
				configs:   []string{previousModesStaticCluster},
				resources: refused,
			},
			"--resources, a copy before the last": {
				configs:   []string{previousModesStaticCluster + previousModesWithoutRegistry},
				resources: refused + previousModesWithoutRegistry,
			},
			"commander's init resources, between the configuration and the resources": {
				configs: []string{
					previousModesStaticCluster + previousModesWithoutRegistry,
					refused,
					previousModesWithoutRegistry,
				},
			},
		}

		for name, tt := range cases {
			t.Run(mode+", "+name, func(t *testing.T) {
				t.Parallel()

				pec := &finalizingPEC{}
				b := &ClusterBootstrapper{Params: &Params{Options: &options.Options{}}, PhasedExecutionContext: pec}
				for i, cfg := range tt.configs {
					b.Options.Global.ConfigPaths = append(b.Options.Global.ConfigPaths,
						writePreviousModesFile(t, "config"+string(rune('0'+i))+".yaml", cfg))
				}
				if tt.resources != "" {
					b.Options.Bootstrap.ResourcesPath = writePreviousModesFile(t, "resources.yaml", tt.resources)
				}

				err := b.Bootstrap(context.Background())
				require.Error(t, err)
				require.Contains(t, err.Error(), `registry.mode "`+mode+`"`)
				require.Contains(t, err.Error(), "belongs to the previous registry implementation")

				require.Equal(t, []phases.OperationPhase{phases.PreparationPhase}, pec.announcedPhases(),
					"refused in Preparation: no later phase may be announced, base infrastructure included")
				require.NotContains(t, pec.completedSubPhases, phases.PreparationSubPhaseConfigValidation)
			})
		}
	}
}

// TestCreateResourcesRefusesThePreviousRegistryImplementation covers the one bootstrap phase that
// reads the resources without parsing the configuration: dhctl bootstrap-phase create-resources.
// The refusal comes before the cluster is connected to, so the bootstrapper here has no kube
// provider at all.
func TestCreateResourcesRefusesThePreviousRegistryImplementation(t *testing.T) {
	t.Parallel()

	for mode, refused := range map[string]string{"Proxy": previousModesProxy, "Local": previousModesLocal} {
		t.Run(mode+", --resources", func(t *testing.T) {
			t.Parallel()

			b := &ClusterBootstrapper{Params: &Params{Options: &options.Options{}}}
			b.Options.Bootstrap.ResourcesPath = writePreviousModesFile(t, "resources.yaml", previousModesWithoutRegistry+refused)

			err := b.CreateResources(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), `registry.mode "`+mode+`"`)
			require.Contains(t, err.Error(), "belongs to the previous registry implementation")
		})

		t.Run(mode+", --config", func(t *testing.T) {
			t.Parallel()

			b := &ClusterBootstrapper{Params: &Params{Options: &options.Options{}}}
			b.Options.Global.ConfigPaths = []string{
				writePreviousModesFile(t, "config.yaml", previousModesStaticCluster+previousModesWithoutRegistry),
				writePreviousModesFile(t, "resources.yaml", refused),
			}

			err := b.CreateResources(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), `registry.mode "`+mode+`"`)
		})
	}
}
