// Copyright 2025 Flant JSC
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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func getTestFSDIParams() *fs.DIParams {
	return &fs.DIParams{
		InfraVersionsFile: "/deckhouse/candi/terraform_versions.yml",
		BinariesDir:       "/dhctl-tests/bin",
		CloudProviderDir:  "/deckhouse/candi/cloud-providers",
		PluginsDir:        "/dhctl-tests/plugins",
	}
}

func getTestCloudProviderGetterParams(t *testing.T, testName string) CloudProviderGetterParams {
	id, err := uuid.NewRandom()
	require.NoError(t, err)

	tmpDir := filepath.Join(os.TempDir(), "dhctl-tests", id.String(), testName)
	err = os.MkdirAll(tmpDir, 0o777)
	require.NoError(t, err)

	logger := log.GetDefaultLogger()
	logger.LogInfoF("Tmp dir for test %s is %s\n", testName, tmpDir)

	return CloudProviderGetterParams{
		TmpDir:           tmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           log.GetDefaultLogger(),
		FSDIParams:       getTestFSDIParams(),
	}
}

func testCleanup(t *testing.T, testName string, params *CloudProviderGetterParams) {
	tmpDir := path.Clean(params.TmpDir)
	require.NotEmpty(t, tmpDir, testName)
	require.NotEqual(t, tmpDir, "/", testName)

	params.Logger.LogInfoF("Cleanup for test %s. Remove tmp dir %s\n", testName, tmpDir)

	err := os.RemoveAll(tmpDir)
	if err != nil {
		t.Fatal(fmt.Errorf("cannot cleaning up tmp dir %s for test %s: %v", tmpDir, testName, err))
	}

	params.Logger.LogInfoF("Test %s cleaned\n", testName)
}

func TestFailCloudProviderGet(t *testing.T) {
	testName := "TestFailCloudProviderGet"

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	// no uuid for cluster
	cfg := &config.MetaConfig{}
	cfg.ProviderName = yandex.ProviderName

	_, err := getter(context.TODO(), cfg)
	require.Error(t, err)

	// incorrect provider
	cfg.ProviderName = "incorrect"
	cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d098"
	_, err = getter(context.TODO(), cfg)
	require.Error(t, err)
}

func TestCloudProviderGet(t *testing.T) {
	testName := "TestCloudProviderGet"

	getMetaConfig := func(t *testing.T, providerName string) *config.MetaConfig {
		cfg := &config.MetaConfig{}
		cfg.ProviderName = providerName
		if providerName != "" {
			cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d098"
		}

		return cfg
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	provider, err := getter(context.TODO(), getMetaConfig(t, ""))
	require.NoError(t, err)
	require.IsType(t, &infrastructure.DummyCloudProvider{}, provider, "provider should be a DummyCloudProvider for static cluster")

	// tofu provider
	providerYandex, err := getter(context.TODO(), getMetaConfig(t, yandex.ProviderName))
	require.NoError(t, err)
	require.IsType(t, &cloud.Provider{}, providerYandex, "provider should be a cloud.Provider for yandex cluster")
	require.True(t, providerYandex.NeedToUseTofu())
	require.Equal(t, providerYandex.Name(), yandex.ProviderName)

	providerVCD, err := getter(context.TODO(), getMetaConfig(t, vcd.ProviderName))
	require.NoError(t, err)
	require.IsType(t, &cloud.Provider{}, providerVCD, "provider should be a cloud.Provider for VCD cluster")
	require.False(t, providerVCD.NeedToUseTofu())
	require.Equal(t, providerVCD.Name(), vcd.ProviderName)
}
