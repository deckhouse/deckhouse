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

package infrastructureprovider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/gcp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestPipelineGetMasterOutputsNoStrict(t *testing.T) {
	testName := "TestPipelineGetMasterOutputsNoStrict"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skipf("Skipping %s test", testName)
	}

	getMetaConfig := func(t *testing.T, providerName, layout string) *config.MetaConfig {
		cfg := &config.MetaConfig{}
		cfg.ProviderName = providerName
		cfg.ClusterPrefix = "test"
		cfg.Layout = layout
		if providerName != "" {
			cfg.UUID = "fb6dfa1c-93fd-11f0-9697-efd55958d098"
		}

		return cfg
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	ctx := context.TODO()

	type testCase struct {
		name      string
		hasError  bool
		statePath string
		expected  *infrastructure.PipelineOutputs
	}

	assertOutputs := func(c testCase, cfg *config.MetaConfig, e infrastructure.Executor) {
		runner := infrastructure.NewRunner(cfg, &cache.DummyCache{}, e)

		fullPath, err := filepath.Abs(c.statePath)
		require.NoError(t, err, "abs path")

		runner.WithStatePath(fullPath)

		outputs, err := infrastructure.GetMasterNodeResultNoStrict(ctx, runner)
		if c.hasError {
			require.Error(t, err, "should error")
			return
		}

		require.NoError(t, err, "should not error")

		stateContent, err := os.ReadFile(c.statePath)
		if err != nil {
			if !os.IsNotExist(err) {
				require.NoError(t, err, "should not error read state file")
			}

			stateContent = make([]byte, 0)
		}

		c.expected.InfrastructureState = stateContent

		require.Equal(t, c.expected, outputs, "should return correct outputs")
	}

	fullExpected := &infrastructure.PipelineOutputs{
		KubeDataDevicePath: "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_f4e372d3da04382b9b4ced77976fa66e",
		MasterIPForSSH:     "1.1.1.1",
		NodeInternalIP:     "10.12.0.112",
	}

	cases := []testCase{
		{
			name:      "all present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/all_present.json",
			expected:  fullExpected,
		},
		{
			name:      "kube path not present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/kube_path_not_present.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     fullExpected.MasterIPForSSH,
				NodeInternalIP:     fullExpected.NodeInternalIP,
			},
		},
		{
			name:      "ssh not present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/ssh_not_present.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: fullExpected.KubeDataDevicePath,
				MasterIPForSSH:     "",
				NodeInternalIP:     fullExpected.NodeInternalIP,
			},
		},
		{
			name:      "node internal not present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/node_internal_not_present.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: fullExpected.KubeDataDevicePath,
				MasterIPForSSH:     fullExpected.MasterIPForSSH,
				NodeInternalIP:     "",
			},
		},
		{
			name:      "node internal only present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/node_internal_only_present.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     "",
				NodeInternalIP:     fullExpected.NodeInternalIP,
			},
		},
		{
			name:      "kube path only present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/kube_path_only_present.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: fullExpected.KubeDataDevicePath,
				MasterIPForSSH:     "",
				NodeInternalIP:     "",
			},
		},
		{
			name:      "ssh ip only present",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/ssh_only_present.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     fullExpected.MasterIPForSSH,
				NodeInternalIP:     "",
			},
		},
		{
			name:      "no outputs",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/no_outputs.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     "",
				NodeInternalIP:     "",
			},
		},
		{
			name:      "no outputs key",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/no_outputs_key.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     "",
				NodeInternalIP:     "",
			},
		},
		{
			name:      "empty state",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/empty.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     "",
				NodeInternalIP:     "",
			},
		},

		{
			name:      "not exists file",
			hasError:  false,
			statePath: "./mocks/pipeline/nostrict/not_exists_yg65.json",
			expected: &infrastructure.PipelineOutputs{
				KubeDataDevicePath: "",
				MasterIPForSSH:     "",
				NodeInternalIP:     "",
			},
		},
		{
			name:      "incorrect state",
			hasError:  true,
			statePath: "./mocks/pipeline/nostrict/incorrect_state.json",
		},
	}

	t.Run("Tofu", func(t *testing.T) {
		providerYandexMetaConfig := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
		providerYandex, err := getter(ctx, providerYandexMetaConfig)
		require.NoError(t, err, "should provide meta config")

		executor, err := providerYandex.Executor(ctx, infrastructure.MasterNodeStep, params.Logger)
		require.NoError(t, err, "should create executor")

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				assertOutputs(c, providerYandexMetaConfig, executor)
			})
		}
	})

	t.Run("Terraform", func(t *testing.T) {
		cfgProviderGCP := getMetaConfig(t, gcp.ProviderName, gcpTestLayout)
		providerGCP, err := getter(ctx, cfgProviderGCP)
		require.NoError(t, err, "should provide meta config")

		executor, err := providerGCP.Executor(ctx, infrastructure.MasterNodeStep, params.Logger)
		require.NoError(t, err, "should create executor")

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				assertOutputs(c, cfgProviderGCP, executor)
			})
		}
	})
}
