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
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/gcp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

const (
	yandexTestLayout = "without-nat"
	gcpTestLayout    = "without-nat"
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

	hash := stringsutil.Sha256Encode(id.String() + testName)
	first8Runes := fmt.Sprintf("%.8s", hash)

	tmpDir := filepath.Join(os.TempDir(), "dhctl-tests", first8Runes)
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

	cfg.ProviderName = yandex.ProviderName

	// empty prefix
	_, err = getter(context.TODO(), cfg)
	require.Error(t, err)

	// empty layout
	cfg.ClusterPrefix = "test"
	_, err = getter(context.TODO(), cfg)
	require.Error(t, err)
}

func TestCloudProviderGetForStatic(t *testing.T) {
	testName := "TestCloudProviderGetForStatic"

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	cfg := &config.MetaConfig{}
	cfg.ProviderName = ""

	provider, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	require.IsType(t, &infrastructure.DummyCloudProvider{}, provider, "provider should be a DummyCloudProvider for static cluster")
}

func TestCloudProviderGet(t *testing.T) {
	testName := "TestCloudProviderGet"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	getMetaConfig := func(t *testing.T, providerName, layout string) *config.MetaConfig {
		cfg := &config.MetaConfig{}
		cfg.ProviderName = providerName
		cfg.ClusterPrefix = "test"
		cfg.Layout = layout
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

	// tofu provider
	providerYandex, err := getter(context.TODO(), getMetaConfig(t, yandex.ProviderName, yandexTestLayout))
	require.NoError(t, err)

	require.IsType(t, &cloud.Provider{}, providerYandex, "provider should be a cloud.Provider for yandex cluster")
	require.True(t, providerYandex.NeedToUseTofu())
	require.Equal(t, providerYandex.Name(), yandex.ProviderName)
	require.True(t, strings.HasSuffix(providerYandex.RootDir(), "infra/e688020e4bc6a1cf"))

	cfgForYandexWithDifferentPrefix := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	cfgForYandexWithDifferentPrefix.ClusterPrefix = "another"
	providerYandexWithAnotherPrefix, err := getter(context.TODO(), cfgForYandexWithDifferentPrefix)
	require.NoError(t, err)
	require.NotEqual(t, providerYandexWithAnotherPrefix.RootDir(), providerYandex.RootDir())

	cfgForYandexWithDifferentUUID := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	cfgForYandexWithDifferentUUID.UUID = "01204020-9407-11f0-a6e3-6747832ba8ef"
	providerYandexWithAnotherUUID, err := getter(context.TODO(), cfgForYandexWithDifferentUUID)
	require.NoError(t, err)
	require.NotEqual(t, providerYandexWithAnotherUUID.RootDir(), providerYandex.RootDir())

	// terraform provider
	providerGCP, err := getter(context.TODO(), getMetaConfig(t, gcp.ProviderName, gcpTestLayout))
	require.NoError(t, err)

	require.NotEqual(t, providerGCP.RootDir(), providerYandex.RootDir())
	require.IsType(t, &cloud.Provider{}, providerGCP, "provider should be a cloud.Provider for GCP cluster")
	require.False(t, providerGCP.NeedToUseTofu())
	require.Equal(t, providerGCP.Name(), gcp.ProviderName)
	require.True(t, strings.HasSuffix(providerGCP.RootDir(), "infra/72ce5a172c9b8efa"))
}

func TestCloudProviderWithTofuExecutorsGetting(t *testing.T) {
	testName := "TestCloudProviderWithTofuExecutorsGetting"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	cfg := &config.MetaConfig{}
	cfg.ProviderName = yandex.ProviderName
	cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d099"
	cfg.ClusterPrefix = "test"
	cfg.Layout = yandexTestLayout

	providerYandex, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	require.IsType(t, &cloud.Provider{}, providerYandex, "provider should be a cloud.Provider for yandex cluster")
	require.True(t, providerYandex.NeedToUseTofu())
	require.Equal(t, providerYandex.Name(), yandex.ProviderName)

	_, err = providerYandex.Executor(context.TODO(), infrastructure.BaseInfraStep, params.Logger)
	require.NoError(t, err)
}
