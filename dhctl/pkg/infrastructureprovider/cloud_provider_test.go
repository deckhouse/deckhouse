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
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/fsprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/gcp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

const (
	yandexTestLayout    = "without-nat"
	yandexPluginVersion = "0.83.0"

	gcpTestLayout    = "without-nat"
	gcpPluginVersion = "3.48.0"

	modulesRootDir = "modules"
	layoutsRootDir = "layouts"
	lockFile       = ".terraform.lock.hcl"

	tofuBin      = "opentofu"
	terraformBin = "terraform"
)

var (
	yandexPluginsDir = []string{
		fmt.Sprintf("registry.opentofu.org/yandex-cloud/yandex/%s/linux_amd64/terraform-provider-yandex", yandexPluginVersion),
	}
	gcpPluginsDir = []string{
		fmt.Sprintf("registry.terraform.io/hashicorp/google/%s/linux_amd64/terraform-provider-google", gcpPluginVersion),
	}
)

func getTestFSDIParams() *fsprovider.DIParams {
	return &fsprovider.DIParams{
		InfraVersionsFile: "/deckhouse/candi/terraform_versions.yml",
		BinariesDir:       "/dhctl-tests/bin",
		CloudProviderDir:  "/deckhouse/candi/cloud-providers",
		PluginsDir:        "/dhctl-tests/plugins",
	}
}

func getTestCloudProviderGetterParams(t *testing.T, testName string) CloudProviderGetterParams {
	t.Helper()

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
		Logger:           logger,
		FSDIParams:       getTestFSDIParams(),
		IsDebug:          false,
		ProvidersCache:   newCloudProvidersMapCache(),
	}
}

func testCleanup(t *testing.T, testName string, params *CloudProviderGetterParams) {
	t.Helper()

	tmpDir := path.Clean(params.TmpDir)
	require.NotEmpty(t, tmpDir, testName)
	require.NotEqual(t, tmpDir, "/", testName)
	require.False(t, govalue.IsNil(params.IsDebug))

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
	assertProvidersCacheIsEmpty(t, params.ProvidersCache)

	// incorrect provider
	cfg.ProviderName = "incorrect"
	cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d098"
	_, err = getter(context.TODO(), cfg)
	require.Error(t, err)
	assertProvidersCacheIsEmpty(t, params.ProvidersCache)

	cfg.ProviderName = yandex.ProviderName

	// empty prefix
	_, err = getter(context.TODO(), cfg)
	require.Error(t, err)
	assertProvidersCacheIsEmpty(t, params.ProvidersCache)

	// empty layout
	cfg.ClusterPrefix = "test"
	_, err = getter(context.TODO(), cfg)
	require.Error(t, err)
	assertProvidersCacheIsEmpty(t, params.ProvidersCache)
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
	cfg.UUID = "e97b98de-97d5-11f0-8047-1f15a06cf89b"

	provider, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	require.IsType(t, &infrastructure.DummyCloudProvider{}, provider, "provider should be a DummyCloudProvider for static cluster")
	// do not cache providers for static
	assertProvidersCacheIsEmpty(t, params.ProvidersCache)
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
	providerYandexMetaConfig := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	providerYandex, err := getter(context.TODO(), providerYandexMetaConfig)
	require.NoError(t, err)

	assertCloudProvider(t, providerYandex, yandex.ProviderName, true)
	require.True(t, strings.HasSuffix(providerYandex.RootDir(), "infra/e688020e4bc6a1cf"))
	providerInClusterParams := testProviderInCacheParams{
		metaConfig: providerYandexMetaConfig,
		provider:   providerYandex,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
	}
	assertProviderForClusterInCache(t, providerInClusterParams)
	assertGetProviderFromCache(t, providerInClusterParams)

	cfgForYandexWithDifferentPrefix := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	cfgForYandexWithDifferentPrefix.ClusterPrefix = "another"
	providerYandexWithAnotherPrefix, err := getter(context.TODO(), cfgForYandexWithDifferentPrefix)
	require.NoError(t, err)
	require.NotEqual(t, providerYandexWithAnotherPrefix.RootDir(), providerYandex.RootDir())
	providerInClusterParams = testProviderInCacheParams{
		metaConfig: cfgForYandexWithDifferentPrefix,
		provider:   providerYandexWithAnotherPrefix,
		cache:      params.ProvidersCache,
		cacheLen:   2,
		logger:     params.Logger,
	}
	assertProviderForClusterInCache(t, providerInClusterParams)
	assertGetProviderFromCache(t, providerInClusterParams)

	cfgForYandexWithDifferentUUID := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	cfgForYandexWithDifferentUUID.UUID = "01204020-9407-11f0-a6e3-6747832ba8ef"
	providerYandexWithAnotherUUID, err := getter(context.TODO(), cfgForYandexWithDifferentUUID)
	require.NoError(t, err)
	require.NotEqual(t, providerYandexWithAnotherUUID.RootDir(), providerYandex.RootDir())
	providerInClusterParams = testProviderInCacheParams{
		metaConfig: cfgForYandexWithDifferentUUID,
		provider:   providerYandexWithAnotherUUID,
		cache:      params.ProvidersCache,
		cacheLen:   3,
		logger:     params.Logger,
	}
	assertProviderForClusterInCache(t, providerInClusterParams)
	assertGetProviderFromCache(t, providerInClusterParams)

	// terraform provider
	cfgProviderGCP := getMetaConfig(t, gcp.ProviderName, gcpTestLayout)
	providerGCP, err := getter(context.TODO(), cfgProviderGCP)
	require.NoError(t, err)

	assertCloudProvider(t, providerGCP, gcp.ProviderName, false)
	require.True(t, strings.HasSuffix(providerGCP.RootDir(), "infra/72ce5a172c9b8efa"))
	require.NotEqual(t, providerGCP.RootDir(), providerYandex.RootDir())
	providerInClusterParams = testProviderInCacheParams{
		metaConfig: cfgProviderGCP,
		provider:   providerGCP,
		cache:      params.ProvidersCache,
		cacheLen:   4,
		logger:     params.Logger,
	}
	assertProviderForClusterInCache(t, providerInClusterParams)
	assertGetProviderFromCache(t, providerInClusterParams)

	// do not get not existing providers and do not erase any providers
	notExistsMetaConfigYandex := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	notExistsMetaConfigYandex.ClusterPrefix = "not-exists"
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: notExistsMetaConfigYandex,
		cache:      params.ProvidersCache,
		cacheLen:   4,
		logger:     params.Logger,
	})

	notExistsMetaConfigGCP := getMetaConfig(t, gcp.ProviderName, gcpTestLayout)
	notExistsMetaConfigGCP.ClusterPrefix = "not-exists"
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: notExistsMetaConfigGCP,
		cache:      params.ProvidersCache,
		cacheLen:   4,
		logger:     params.Logger,
	})
}

func TestDefaultCloudProvidersCache(t *testing.T) {
	testName := "TestDefaultCloudProvidersCache"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	getMetaConfig := func(t *testing.T, providerName, layout string) *config.MetaConfig {
		cfg := &config.MetaConfig{}
		cfg.ProviderName = providerName
		cfg.ClusterPrefix = "test"
		cfg.Layout = layout
		if providerName != "" {
			cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d798"
		}

		return cfg
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	params.ProvidersCache = nil

	getter := CloudProviderGetter(params)

	// tofu provider
	providerYandexMetaConfig := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	providerYandex, err := getter(context.TODO(), providerYandexMetaConfig)
	require.NoError(t, err)

	assertCloudProvider(t, providerYandex, yandex.ProviderName, true)
	providerInClusterParams := testProviderInCacheParams{
		metaConfig: providerYandexMetaConfig,
		provider:   providerYandex,
		cache:      defaultProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
	}
	assertProviderForClusterInCache(t, providerInClusterParams)
	assertGetProviderFromCache(t, providerInClusterParams)

	// terraform provider
	cfgProviderGCP := getMetaConfig(t, gcp.ProviderName, gcpTestLayout)
	providerGCP, err := getter(context.TODO(), cfgProviderGCP)
	require.NoError(t, err)

	assertCloudProvider(t, providerGCP, gcp.ProviderName, false)
	require.NotEqual(t, providerGCP.RootDir(), providerYandex.RootDir())
	providerInClusterParams = testProviderInCacheParams{
		metaConfig: cfgProviderGCP,
		provider:   providerGCP,
		cache:      defaultProvidersCache,
		cacheLen:   2,
		logger:     params.Logger,
	}
	assertProviderForClusterInCache(t, providerInClusterParams)
	assertGetProviderFromCache(t, providerInClusterParams)

	// do not get not existing providers and do not erase any providers
	notExistsMetaConfigYandex := getMetaConfig(t, yandex.ProviderName, yandexTestLayout)
	notExistsMetaConfigYandex.ClusterPrefix = "not-exists"
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: notExistsMetaConfigYandex,
		cache:      defaultProvidersCache,
		cacheLen:   2,
		logger:     params.Logger,
	})

	notExistsMetaConfigGCP := getMetaConfig(t, gcp.ProviderName, gcpTestLayout)
	notExistsMetaConfigGCP.ClusterPrefix = "not-exists"
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: notExistsMetaConfigGCP,
		cache:      defaultProvidersCache,
		cacheLen:   2,
		logger:     params.Logger,
	})

	// cleanup default cache
	err = providerYandex.Cleanup()
	require.NoError(t, err)
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: providerYandexMetaConfig,
		cache:      defaultProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
	})
	// double cleanup does not affect another keys
	err = providerYandex.Cleanup()
	require.NoError(t, err)
	assertProvidersCacheHasCountOfKeys(t, defaultProvidersCache, 1)

	err = providerGCP.Cleanup()
	require.NoError(t, err)
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfgProviderGCP,
		cache:      defaultProvidersCache,
		cacheLen:   0,
		logger:     params.Logger,
	})
	// double cleanup does not affect another keys
	err = providerGCP.Cleanup()
	require.NoError(t, err)
	assertProvidersCacheHasCountOfKeys(t, defaultProvidersCache, 0)

	// cleanup default cache
	providerGCPTmp, err := getter(context.TODO(), cfgProviderGCP)
	require.NoError(t, err)
	// provide new provider but for one cluster
	require.Equal(t, providerGCPTmp.String(), providerGCP.String())
	require.NotEqual(t, providerGCPTmp, providerGCP)

	providerYandexTmp, err := getter(context.TODO(), providerYandexMetaConfig)
	require.NoError(t, err)
	// provide new provider but for one cluster
	require.Equal(t, providerYandexTmp.String(), providerYandex.String())
	require.NotEqual(t, providerYandexTmp, providerYandex)

	assertProvidersCacheHasCountOfKeys(t, defaultProvidersCache, 2)

	loggerProvider := log.SimpleLoggerProvider(params.Logger)

	CleanupProvidersFromDefaultCache(loggerProvider)

	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)

	// do not allow any operations on finalized cache
	_, err = getter(context.TODO(), providerYandexMetaConfig)
	require.Error(t, err)
	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)

	_, err = getter(context.TODO(), cfgProviderGCP)
	require.Error(t, err)
	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)

	_, _, err = defaultProvidersCache.Get(providerYandexMetaConfig.UUID, providerYandexMetaConfig, params.Logger)
	require.Error(t, err)
	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)

	_, err = defaultProvidersCache.GetOrAdd(context.TODO(), providerYandexMetaConfig.UUID, providerYandexMetaConfig, params.Logger, func(context.Context, string, *config.MetaConfig, log.Logger) (infrastructure.CloudProvider, error) {
		return providerYandex, nil
	})
	require.Error(t, err)
	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)

	err = defaultProvidersCache.IterateOverCache(func(key string, provider infrastructure.CloudProvider) {
		require.True(t, false, "Do not allow iterate over finalized default cache")
	})
	require.Error(t, err)
	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)

	// double cleanup cache does not panic
	CleanupProvidersFromDefaultCache(loggerProvider)
	require.Len(t, defaultProvidersCache.cloudProvidersCache, 0)
}

func TestCloudProviderWithTofuExecutorGetting(t *testing.T) {
	testName := "TestCloudProviderWithTofuExecutorGetting"

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
	assertCloudProvider(t, providerYandex, yandex.ProviderName, true)

	step := infrastructure.BaseInfraStep

	executor, err := providerYandex.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)

	assertCorrectExecutorStatesDir(t, executor, providerYandex, yandexPluginVersion)

	versionsContent := fmt.Sprintf(`
terraform {
  required_version = ">= 0.14.8"
  required_providers {
    yandex = {
      source  = "yandex-cloud/yandex"
      version = ">= %s"
    }
  }
}
`, yandexPluginVersion)
	testParams := assertAllFilesCopiedToProviderDirParams{
		provider:        providerYandex,
		versionsContent: versionsContent,
		layouts: []string{
			"standard",
			"with-nat-instance",
			"without-nat",
		},
		usedLayout: yandexTestLayout,
		usedStep:   step,
		modules: []string{
			"master-node",
			"monitoring-service-account",
			"static-node",
			"vpc-components",
		},
		pluginPaths:   yandexPluginsDir,
		pluginVersion: yandexPluginVersion,
	}

	assertAllFilesCopiedToProviderDir(t, testParams, params)

	// executor does not affect cache
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerYandex,
	})

	// does not corrupt if multiple executor get
	_, err = providerYandex.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)

	// does not corrupt is another step used
	anotherStep := infrastructure.MasterNodeStep
	testParams.usedStep = anotherStep
	_, err = providerYandex.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)

	// executor with another step does not affect cache
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerYandex,
	})

	// all steps presents
	staticStep := infrastructure.StaticNodeStep
	testParams.usedStep = staticStep
	_, err = providerYandex.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)
	for _, s := range []infrastructure.Step{step, anotherStep, staticStep} {
		testParams.usedStep = s
		assertAllFilesCopiedToProviderDir(t, testParams, params)
	}

	assertCleanupNotFaultAndKeepFSDIDirsAndFiles(t, providerYandex, params, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		logger:     params.Logger,
	})

	// no cleanup if use debug
	paramsWithDebug := params
	paramsWithDebug.IsDebug = true
	getterDebug := CloudProviderGetter(paramsWithDebug)
	cfg.UUID = "8fff5216-971a-11f0-aa51-f7999b2068b2"
	providerYandexWithDebug, err := getterDebug(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerYandexWithDebug, yandex.ProviderName, true)
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerYandexWithDebug,
	})
	assertKeepRootDirWithDebugAndKeepFSDIDirsAndFiles(t, providerYandexWithDebug, paramsWithDebug)
	// yes we should clean provider from cache in debug mode anyway
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   0,
		logger:     params.Logger,
	})
}

func TestCloudProviderWithTerraformExecutorGetting(t *testing.T) {
	testName := "TestCloudProviderWithTerraformExecutorGetting"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	cfg := &config.MetaConfig{}
	cfg.ProviderName = gcp.ProviderName
	cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d019"
	cfg.ClusterPrefix = "test"
	cfg.Layout = gcpTestLayout

	providerGCP, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerGCP, gcp.ProviderName, false)

	step := infrastructure.BaseInfraStep

	executor, err := providerGCP.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)

	assertCorrectExecutorStatesDir(t, executor, providerGCP, gcpPluginVersion)

	versionsContent := fmt.Sprintf(`
terraform {
  required_version = ">= 0.14.8"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= %s"
    }
  }
}
`, gcpPluginVersion)

	testParams := assertAllFilesCopiedToProviderDirParams{
		provider:        providerGCP,
		versionsContent: versionsContent,
		layouts: []string{
			"standard",
			"without-nat",
		},
		usedLayout: gcpTestLayout,
		usedStep:   step,
		modules: []string{
			"base-infrastructure",
			"firewall",
			"master-node",
			"static-node",
		},
		pluginPaths:   gcpPluginsDir,
		pluginVersion: gcpPluginVersion,
	}

	assertAllFilesCopiedToProviderDir(t, testParams, params)

	// does not corrupt if multiple executor get
	_, err = providerGCP.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)

	// executor does not affect cache
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerGCP,
	})

	// does not corrupt is another step used
	anotherStep := infrastructure.MasterNodeStep
	testParams.usedStep = anotherStep
	_, err = providerGCP.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)

	// executor does not affect cache for another step
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerGCP,
	})

	// all steps presents
	staticStep := infrastructure.StaticNodeStep
	testParams.usedStep = staticStep
	_, err = providerGCP.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)
	for _, s := range []infrastructure.Step{step, anotherStep, staticStep} {
		testParams.usedStep = s
		assertAllFilesCopiedToProviderDir(t, testParams, params)
	}

	assertCleanupNotFaultAndKeepFSDIDirsAndFiles(t, providerGCP, params, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		logger:     params.Logger,
	})

	// no cleanup if use debug
	paramsWithDebug := params
	paramsWithDebug.IsDebug = true
	getterDebug := CloudProviderGetter(paramsWithDebug)
	cfg.UUID = "8fff5216-971a-11f0-aa51-f7999b2068b3"
	providerGCPWithDebug, err := getterDebug(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerGCPWithDebug, gcp.ProviderName, false)
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerGCPWithDebug,
	})
	assertKeepRootDirWithDebugAndKeepFSDIDirsAndFiles(t, providerGCPWithDebug, paramsWithDebug)
	// yes we should clean provider from cache in debug mode anyway
	assertDoesNotGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   0,
		logger:     params.Logger,
	})
}

func TestCloudProviderWithTofuOutputExecutorGetting(t *testing.T) {
	testName := "TestCloudProviderWithTofuOutputExecutorGetting"

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
	cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d029"
	cfg.ClusterPrefix = "test"
	cfg.Layout = yandexTestLayout

	providerYandex, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerYandex, yandex.ProviderName, true)

	_, err = providerYandex.OutputExecutor(context.TODO(), params.Logger)
	require.NoError(t, err)
	// output executor does not affect cache
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerYandex,
	})

	assertAllFilesCopiedToProviderDirForOutputExecutor(t, providerYandex, params)
}

func TestCloudProviderWithTerraformOutputExecutorGetting(t *testing.T) {
	testName := "TestCloudProviderWithTerraformOutputExecutorGetting"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	getter := CloudProviderGetter(params)

	cfg := &config.MetaConfig{}
	cfg.ProviderName = gcp.ProviderName
	cfg.UUID = "fb6dfacc-93fd-11f0-9697-efd55958d119"
	cfg.ClusterPrefix = "test"
	cfg.Layout = gcpTestLayout

	providerGCP, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerGCP, gcp.ProviderName, false)

	_, err = providerGCP.OutputExecutor(context.TODO(), params.Logger)
	require.NoError(t, err)
	// output executor does not affect cache
	assertGetProviderFromCache(t, testProviderInCacheParams{
		metaConfig: cfg,
		cache:      params.ProvidersCache,
		cacheLen:   1,
		logger:     params.Logger,
		provider:   providerGCP,
	})

	assertAllFilesCopiedToProviderDirForOutputExecutor(t, providerGCP, params)
}

func TestTofuInitAndPlanWithCreatingWorkerFilesInRoot(t *testing.T) {
	testName := "TestTofuInitAndPlanWithCreatingWorkerFilesInRoot"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	cfg := provideTestMetaConfig(t, testProvideMetaConfigParams{
		env:      "YANDEX_CLOUD_CONFIG",
		testName: testName,
		layout:   yandexTestLayout,
		uuid:     "e0bcfdc2-95a1-11f0-987b-234a7238ed8d",
		logger:   params.Logger,
	})

	getter := CloudProviderGetter(params)

	providerYandex, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerYandex, yandex.ProviderName, true)

	assertExecInitAndPlanResults(t, execInitAndPlanResultsParams{
		provider: providerYandex,
		step:     infrastructure.BaseInfraStep,
		params:   params,
		configProvider: func() []byte {
			return cfg.MarshalConfig()
		},
		layout:        yandexTestLayout,
		pluginsDir:    yandexPluginsDir,
		pluginVersion: yandexPluginVersion,
	})

	assertExecInitAndPlanResults(t, execInitAndPlanResultsParams{
		provider: providerYandex,
		step:     infrastructure.MasterNodeStep,
		params:   params,
		configProvider: func() []byte {
			return cfg.NodeGroupConfig("master", 0, "")
		},
		layout:        yandexTestLayout,
		pluginsDir:    yandexPluginsDir,
		pluginVersion: yandexPluginVersion,
	})

	assertExecInitAndPlanResults(t, execInitAndPlanResultsParams{
		provider: providerYandex,
		step:     infrastructure.StaticNodeStep,
		params:   params,
		configProvider: func() []byte {
			return cfg.NodeGroupConfig("worker", 0, "")
		},
		layout:        yandexTestLayout,
		pluginsDir:    yandexPluginsDir,
		pluginVersion: yandexPluginVersion,
	})
}

func TestTerraformInitAndPlanWithCreatingWorkerFilesInRoot(t *testing.T) {
	testName := "TestTerraformInitAndPlanWithCreatingWorkerFilesInRoot"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	cfg := provideTestMetaConfig(t, testProvideMetaConfigParams{
		env:      "GCP_CLOUD_CONFIG",
		testName: testName,
		layout:   gcpTestLayout,
		uuid:     "e0bcfdc2-95a1-11f0-987b-234a7238ed8c",
		logger:   params.Logger,
	})

	getter := CloudProviderGetter(params)

	providerGCP, err := getter(context.TODO(), cfg)
	require.NoError(t, err)
	assertCloudProvider(t, providerGCP, gcp.ProviderName, false)

	assertExecInitAndPlanResults(t, execInitAndPlanResultsParams{
		provider: providerGCP,
		step:     infrastructure.BaseInfraStep,
		params:   params,
		configProvider: func() []byte {
			return cfg.MarshalConfig()
		},
		layout:        gcpTestLayout,
		pluginsDir:    gcpPluginsDir,
		pluginVersion: gcpPluginVersion,
	})

	assertExecInitAndPlanResults(t, execInitAndPlanResultsParams{
		provider: providerGCP,
		step:     infrastructure.MasterNodeStep,
		params:   params,
		configProvider: func() []byte {
			return cfg.NodeGroupConfig("master", 0, "")
		},
		layout:        gcpTestLayout,
		pluginsDir:    gcpPluginsDir,
		pluginVersion: gcpPluginVersion,
	})

	assertExecInitAndPlanResults(t, execInitAndPlanResultsParams{
		provider: providerGCP,
		step:     infrastructure.StaticNodeStep,
		params:   params,
		configProvider: func() []byte {
			return cfg.NodeGroupConfig("worker", 0, "")
		},
		layout:        gcpTestLayout,
		pluginsDir:    gcpPluginsDir,
		pluginVersion: gcpPluginVersion,
	})
}

func TestTofuApplyWithCreatingWorkerFilesInRoot(t *testing.T) {
	testName := "TestTofuApplyWithCreatingWorkerFilesInRoot"

	if os.Getenv("SKIP_PROVIDER_TEST") == "true" {
		t.Skip(fmt.Sprintf("Skipping %s test", testName))
	}

	params := getTestCloudProviderGetterParams(t, testName)
	defer func() {
		testCleanup(t, testName, &params)
	}()

	assertApplyWithCreatingWorkerFilesInRoot(t, assertApplyWithCreatingWorkerFilesInRootParams{
		params:        params,
		provider:      yandex.ProviderName,
		pluginVersion: yandexPluginVersion,
		useTofu:       true,
		pluginsDir:    yandexPluginsDir,
	})
}

type assertApplyWithCreatingWorkerFilesInRootParams struct {
	params                  CloudProviderGetterParams
	provider, pluginVersion string
	useTofu                 bool
	pluginsDir              []string
}

func assertApplyWithCreatingWorkerFilesInRoot(t *testing.T, params assertApplyWithCreatingWorkerFilesInRootParams) {
	require.NotEmpty(t, params.provider)
	require.NotEmpty(t, params.pluginVersion)
	require.NotEmpty(t, params.pluginsDir)

	applyStep := infrastructure.BaseInfraStep
	applyLayout := "fake"

	cfgApply := fakeApplyTestMetaConfig(t, fakeApplyTestMetaConfigParams{
		layout:   applyLayout,
		uuid:     "f04bd5fa-998a-11f0-98c9-83a48e5f683f",
		provider: params.provider,
	})

	getter := CloudProviderGetter(params.params)

	applyProvider, err := getter(context.TODO(), cfgApply)

	_, cleanup := testPrepareFakeLayoutForApply(t, testPrepareFakeLayoutForApplyParams{
		provider: applyProvider,
		step:     applyStep,
		layout:   applyLayout,
	}, params.params)

	defer cleanup(t)

	assertCloudProvider(t, applyProvider, params.provider, params.useTofu)
	require.NoError(t, err)

	execParams := execInitAndPlanResultsParams{
		provider: applyProvider,
		step:     applyStep,
		params:   params.params,
		configProvider: func() []byte {
			return []byte("{}")
		},
		layout:        applyLayout,
		pluginsDir:    params.pluginsDir,
		pluginVersion: params.pluginVersion,
	}

	executorForApply, planParams := assertExecInitAndPlanResults(t, execParams)

	err = executorForApply.Apply(context.TODO(), infrastructure.ApplyOpts{
		StatePath:     planParams.StatePath,
		PlanPath:      planParams.OutPath,
		VariablesPath: planParams.VariablesPath,
	})

	require.NoError(t, err)

	assertTerraformDirNotExistsInHomeAndPresentInInfraRoot(t, execParams)
}

type testPrepareFakeLayoutForApplyParams struct {
	layout   string
	provider infrastructure.CloudProvider
	step     infrastructure.Step
}

func testPrepareFakeLayoutForApply(t *testing.T, params testPrepareFakeLayoutForApplyParams, cloudParams CloudProviderGetterParams) (string, func(t *testing.T)) {
	require.False(t, govalue.IsNil(params.provider))
	require.NotEmpty(t, params.layout)
	require.NotEmpty(t, params.step)
	require.NotNil(t, cloudParams.FSDIParams)

	step := string(params.step)

	fakeLayoutDir := filepath.Join(
		cloudParams.FSDIParams.CloudProviderDir,
		params.provider.Name(),
		layoutsRootDir,
		params.layout,
	)

	fakeStepDir := filepath.Join(fakeLayoutDir, step)

	err := os.MkdirAll(fakeStepDir, 0o777)
	require.NoError(t, err)

	cloudParams.Logger.LogInfoF("Fake layout dir %s created\n", fakeLayoutDir)

	cleanup := func(tt *testing.T) {
		err := os.RemoveAll(fakeLayoutDir)
		require.NoError(tt, err)
		cloudParams.Logger.LogInfoF("Fake layout dir %s removed\n", fakeLayoutDir)
	}

	infraBin := filepath.Join(cloudParams.FSDIParams.BinariesDir, getProviderInfraUtilBinary(t, params.provider))

	fakeResources := fmt.Sprintf(`
resource "terraform_data" "example" {
  provisioner "local-exec" {
    command = "%s --version"
  }
}
`, infraBin)

	resourcesPath := filepath.Join(fakeStepDir, "main.tf")

	err = os.WriteFile(resourcesPath, []byte(fakeResources), 0o777)
	if err != nil {
		cleanup(t)
		require.NoError(t, err)
	}

	return fakeLayoutDir, cleanup
}

func getCacheKeyForCluster(metaConfig *config.MetaConfig) string {
	return fmt.Sprintf(
		"%s/%s/%s/%s",
		metaConfig.ClusterPrefix,
		metaConfig.UUID,
		metaConfig.ProviderName,
		metaConfig.Layout,
	)
}

func getTestProvidersCacheEntries(t *testing.T, cache CloudProvidersCache) map[string]infrastructure.CloudProvider {
	t.Helper()

	require.False(t, govalue.IsNil(cache))

	entries := make(map[string]infrastructure.CloudProvider)

	err := cache.IterateOverCache(func(key string, provider infrastructure.CloudProvider) {
		entries[key] = provider
	})

	require.NoError(t, err)

	return entries
}

type testProviderInCacheParams struct {
	metaConfig *config.MetaConfig
	provider   infrastructure.CloudProvider
	cache      CloudProvidersCache
	logger     log.Logger
	cacheLen   int
}

func assertGetProviderFromCache(t *testing.T, params testProviderInCacheParams) {
	t.Helper()

	require.NotNil(t, params.metaConfig)
	require.False(t, govalue.IsNil(params.logger))
	require.False(t, govalue.IsNil(params.cache))
	require.False(t, govalue.IsNil(params.provider))
	require.NotEmpty(t, params.metaConfig.UUID)

	cachedProvider, exists, err := params.cache.Get(params.metaConfig.UUID, params.metaConfig, params.logger)
	require.NoError(t, err)
	require.True(t, exists)
	require.False(t, govalue.IsNil(cachedProvider))

	require.Equal(t, params.provider.String(), cachedProvider.String())
	require.Equal(t, params.provider, cachedProvider)
}

func assertDoesNotGetProviderFromCache(t *testing.T, params testProviderInCacheParams) {
	t.Helper()

	require.NotNil(t, params.metaConfig)
	require.False(t, govalue.IsNil(params.logger))
	require.False(t, govalue.IsNil(params.cache))
	require.True(t, govalue.IsNil(params.provider))
	require.NotEmpty(t, params.metaConfig.UUID)

	cachedProvider, exists, err := params.cache.Get(params.metaConfig.UUID, params.metaConfig, params.logger)
	require.NoError(t, err)
	require.False(t, exists)
	require.True(t, govalue.IsNil(cachedProvider))

	entities := getTestProvidersCacheEntries(t, params.cache)
	require.Len(t, entities, params.cacheLen)
}

func assertProviderForClusterInCache(t *testing.T, params testProviderInCacheParams) {
	t.Helper()

	require.NotNil(t, params.metaConfig)
	require.False(t, govalue.IsNil(params.logger))
	require.False(t, govalue.IsNil(params.cache))
	require.False(t, govalue.IsNil(params.provider))

	entries := getTestProvidersCacheEntries(t, params.cache)
	if params.cacheLen >= 0 {
		require.Len(t, entries, params.cacheLen)
	}

	cacheKey := getCacheKeyForCluster(params.metaConfig)
	require.Contains(t, entries, cacheKey)

	providerInCache := entries[cacheKey]
	require.False(t, govalue.IsNil(providerInCache))

	require.Equal(t, params.provider.String(), providerInCache.String())
	require.Equal(t, params.provider, providerInCache)
}

func assertProvidersCacheIsEmpty(t *testing.T, cache CloudProvidersCache) {
	t.Helper()

	entries := getTestProvidersCacheEntries(t, cache)
	require.Len(t, entries, 0)
}

func assertProvidersCacheHasCountOfKeys(t *testing.T, cache CloudProvidersCache, cacheLen int) {
	t.Helper()

	entries := getTestProvidersCacheEntries(t, cache)
	require.Len(t, entries, cacheLen)
}

func assertCloudProvider(t *testing.T, provider infrastructure.CloudProvider, providerName string, useTofu bool) {
	t.Helper()

	require.False(t, govalue.IsNil(provider))
	require.IsType(t, &cloud.Provider{}, provider, "provider should be a cloud.Provider for", providerName)
	require.Equal(t, provider.NeedToUseTofu(), useTofu)
	require.Equal(t, provider.Name(), providerName)
}

func assertCorrectExecutorStatesDir(t *testing.T, executor infrastructure.Executor, provider infrastructure.CloudProvider, pluginVersion string) {
	t.Helper()

	require.False(t, govalue.IsNil(executor))
	require.False(t, govalue.IsNil(provider))
	require.NotEmpty(t, pluginVersion)

	executorStatesDir := executor.GetStatesDir()
	require.NotEmpty(t, executorStatesDir)
	require.NotEqual(t, path.Clean(executorStatesDir), "/")
	require.NotEqual(t, executorStatesDir, provider.RootDir())
	require.True(t, strings.HasPrefix(executorStatesDir, provider.RootDir()))
	require.Equal(t, executorStatesDir, filepath.Join(provider.RootDir(), pluginVersion))
}

func assertFileExistsAndSymlink(t *testing.T, source string, destination string) {
	t.Helper()

	stat, err := os.Lstat(destination)
	require.NoError(t, err, destination)

	isLink, realPath, err := fs.IsSymlinkFromInfo(destination, stat)

	require.NoError(t, err, destination)
	require.True(t, isLink, destination)
	require.Equal(t, source, realPath)
}

func assertFileExists(t *testing.T, filePath string) {
	t.Helper()

	stat, err := os.Stat(filePath)
	require.NoError(t, err, filePath)

	isLink, _, err := fs.IsSymlinkFromInfo(filePath, stat)

	require.NoError(t, err, filePath)
	require.False(t, isLink, filePath)
	require.False(t, stat.IsDir(), filePath)
}

func assertFileExistsAndHasAnyContent(t *testing.T, filePath string) {
	t.Helper()

	assertFileExists(t, filePath)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, filePath)
	require.True(t, len(content) > 0, filePath)
}

func assertFileExistsAndHasContent(t *testing.T, filePath string, expectedContent string) {
	t.Helper()

	assertFileExists(t, filePath)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, filePath)

	require.Equal(t, expectedContent, string(content), filePath)
}

func assertDirExists(t *testing.T, dirPath string) {
	t.Helper()

	stat, err := os.Stat(dirPath)
	require.NoError(t, err, dirPath)
	require.True(t, stat.IsDir(), dirPath)
}

func assertIsNotEmptyDir(t *testing.T, dirPath string) {
	t.Helper()

	assertDirExists(t, dirPath)

	entries, err := os.ReadDir(dirPath)
	require.NoError(t, err, dirPath)
	require.True(t, len(entries) > 0, dirPath)
}

func assertDirNotExists(t *testing.T, dirPath, msg string) {
	t.Helper()

	_, err := os.Stat(dirPath)
	require.True(t, os.IsNotExist(err), dirPath, msg)
}

func assertFSDIDirsAndFilesExists(t *testing.T, params CloudProviderGetterParams) {
	t.Helper()

	require.NotNil(t, params.FSDIParams)

	assertIsNotEmptyDir(t, params.FSDIParams.PluginsDir)
	assertIsNotEmptyDir(t, params.FSDIParams.BinariesDir)
	assertIsNotEmptyDir(t, params.FSDIParams.CloudProviderDir)
	assertFileExistsAndHasAnyContent(t, params.FSDIParams.InfraVersionsFile)
}

func assertKeepRootDirWithDebugAndKeepFSDIDirsAndFiles(t *testing.T, provider infrastructure.CloudProvider, params CloudProviderGetterParams) {
	t.Helper()

	require.False(t, govalue.IsNil(provider))
	require.False(t, govalue.IsNil(params.Logger))
	require.True(t, params.IsDebug)
	require.NotNil(t, params.FSDIParams)

	const cleanupGroup = "testAfterCleanupDebug"

	anotherCleanupExecutedFirst := false
	provider.AddAfterCleanupFunc(cleanupGroup, func(logger log.Logger) {
		logger.LogInfoLn("Test first AfterCleanup with debug called")
		anotherCleanupExecutedFirst = true
	})

	anotherCleanupExecutedSecond := false
	provider.AddAfterCleanupFunc(cleanupGroup, func(logger log.Logger) {
		logger.LogInfoLn("Test second AfterCleanup with debug called")
		anotherCleanupExecutedSecond = true
	})

	_, err := provider.Executor(context.TODO(), infrastructure.BaseInfraStep, params.Logger)
	require.NoError(t, err)

	err = provider.Cleanup()
	require.NoError(t, err)
	assertIsNotEmptyDir(t, provider.RootDir())
	assertFSDIDirsAndFilesExists(t, params)
	// all cleanup functions called in one group anotherGroup is cache clean do not need test
	require.True(t, anotherCleanupExecutedFirst)
	require.True(t, anotherCleanupExecutedSecond)

	anotherCleanupExecutedFirst = false
	anotherCleanupExecutedSecond = false

	// double cleanup does not provide error
	err = provider.Cleanup()
	require.NoError(t, err)
	assertIsNotEmptyDir(t, provider.RootDir())
	assertFSDIDirsAndFilesExists(t, params)
	// all cleanup functions called in one group anotherGroup is cache clean do not need test
	require.False(t, anotherCleanupExecutedFirst)
	require.False(t, anotherCleanupExecutedSecond)
}

func assertCleanupNotFaultAndKeepFSDIDirsAndFiles(t *testing.T, provider infrastructure.CloudProvider, params CloudProviderGetterParams, cacheParams testProviderInCacheParams) {
	require.False(t, govalue.IsNil(provider))
	require.NotNil(t, params.FSDIParams)

	const cleanupGroup = "testAfterCleanup"

	anotherCleanupExecutedFirst := false
	provider.AddAfterCleanupFunc(cleanupGroup, func(logger log.Logger) {
		logger.LogInfoLn("Test first AfterCleanup without debug called")
		anotherCleanupExecutedFirst = true
	})

	anotherCleanupExecutedSecond := false
	provider.AddAfterCleanupFunc(cleanupGroup, func(logger log.Logger) {
		logger.LogInfoLn("Test second AfterCleanup without debug called")
		anotherCleanupExecutedSecond = true
	})

	// cleanup
	err := provider.Cleanup()
	require.NoError(t, err)
	assertDirNotExists(t, provider.RootDir(), "")
	assertFSDIDirsAndFilesExists(t, params)
	// all cleanup functions called in one group anotherGroup is cache clean do not need test
	require.True(t, anotherCleanupExecutedFirst)
	require.True(t, anotherCleanupExecutedSecond)

	// cleanup cache
	cacheForGetParams := cacheParams
	cacheForGetParams.cacheLen = 0
	assertDoesNotGetProviderFromCache(t, cacheForGetParams)

	anotherCleanupExecutedFirst = false
	anotherCleanupExecutedSecond = false

	// double cleanup does not provide error
	err = provider.Cleanup()
	require.NoError(t, err)
	assertDirNotExists(t, provider.RootDir(), "")
	assertFSDIDirsAndFilesExists(t, params)
	assertDoesNotGetProviderFromCache(t, cacheForGetParams)
	// does not execute additional cleanup
	require.False(t, anotherCleanupExecutedFirst)
	require.False(t, anotherCleanupExecutedSecond)
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	require.Error(t, err, path)
	require.True(t, os.IsNotExist(err), path)
}

type assertAllFilesCopiedToProviderDirParams struct {
	provider infrastructure.CloudProvider

	pluginVersion   string
	versionsContent string
	usedLayout      string
	usedStep        infrastructure.Step

	pluginPaths []string

	layouts []string
	modules []string
}

func getProviderInfraUtilBinary(t *testing.T, provider infrastructure.CloudProvider) string {
	t.Helper()

	require.False(t, govalue.IsNil(provider))

	infraBin := terraformBin
	if provider.NeedToUseTofu() {
		infraBin = tofuBin
	}

	return infraBin
}

func assertInfraUtilCopied(t *testing.T, provider infrastructure.CloudProvider, providerParams CloudProviderGetterParams, pluginVersion string) {
	t.Helper()

	infraBin := getProviderInfraUtilBinary(t, provider)
	infraBinPath := filepath.Join(providerParams.FSDIParams.BinariesDir, infraBin)
	assertFileExistsAndSymlink(t, infraBinPath, filepath.Join(provider.RootDir(), pluginVersion, infraBin))
}

func assertPluginsPresent(t *testing.T, root string, pluginPaths []string, source string) {
	t.Helper()

	for _, pluginPath := range pluginPaths {
		destinationPath := filepath.Join(root, pluginPath)
		sourcePath := filepath.Join(source, pluginPath)
		assertFileExistsAndSymlink(t, sourcePath, destinationPath)
	}
}

func getTestStepDir(root string, step infrastructure.Step, layout string) string {
	return filepath.Join(
		root,
		modulesRootDir,
		layoutsRootDir,
		layout,
		string(step),
	)
}

func assertAllFilesCopiedToProviderDir(t *testing.T, params assertAllFilesCopiedToProviderDirParams, providerParams CloudProviderGetterParams) {
	t.Helper()

	provider := params.provider
	require.False(t, govalue.IsNil(provider))

	require.NotEmpty(t, params.usedStep)
	require.NotEmpty(t, params.usedLayout)
	require.NotEmpty(t, params.versionsContent)
	require.NotEmpty(t, params.pluginVersion)

	assertInfraUtilCopied(t, provider, providerParams, params.pluginVersion)

	require.NotEmpty(t, params.pluginPaths)

	infraRoot := filepath.Join(provider.RootDir(), params.pluginVersion)

	assertPluginsPresent(t,
		filepath.Join(infraRoot, "plugins"),
		params.pluginPaths,
		providerParams.FSDIParams.PluginsDir,
	)

	const versionsFile = "versions.tf"

	versionsFileWithContentPath := filepath.Join(infraRoot, versionsFile)
	assertFileExistsAndHasContent(t, versionsFileWithContentPath, params.versionsContent)

	assertFileNotExists(t, filepath.Join(infraRoot, modulesRootDir, layoutsRootDir, versionsFile))

	require.NotEmpty(t, params.layouts)

	for _, layout := range params.layouts {
		layoutDir := filepath.Join(infraRoot, modulesRootDir, layoutsRootDir, layout)
		assertIsNotEmptyDir(t, layoutDir)
	}

	versionsFileForStep := filepath.Join(
		getTestStepDir(infraRoot, params.usedStep, params.usedLayout),
		versionsFile,
	)

	assertFileExistsAndSymlink(t, versionsFileWithContentPath, versionsFileForStep)

	modulesDir := filepath.Join(infraRoot, modulesRootDir, "terraform-modules")
	assertIsNotEmptyDir(t, modulesDir)
	assertFileExistsAndSymlink(t, versionsFileWithContentPath, filepath.Join(modulesDir, versionsFile))

	require.NotEmpty(t, params.modules)

	for _, module := range params.modules {
		moduleDir := filepath.Join(modulesDir, module)
		assertIsNotEmptyDir(t, moduleDir)
		assertFileExistsAndSymlink(t, versionsFileWithContentPath, filepath.Join(moduleDir, versionsFile))
	}
}

func assertAllFilesCopiedToProviderDirForOutputExecutor(t *testing.T, provider infrastructure.CloudProvider, providerParams CloudProviderGetterParams) {
	t.Helper()

	require.False(t, govalue.IsNil(provider))

	assertInfraUtilCopied(t, provider, providerParams, "")

	entries, err := os.ReadDir(provider.RootDir())
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

type fakeApplyTestMetaConfigParams struct {
	layout, uuid, provider string
}

func fakeApplyTestMetaConfig(t *testing.T, params fakeApplyTestMetaConfigParams) *config.MetaConfig {
	require.NotEmpty(t, params.layout)
	require.NotEmpty(t, params.provider)
	require.NotEmpty(t, params.uuid)

	cfg := &config.MetaConfig{}
	cfg.UUID = params.uuid
	cfg.ProviderName = params.provider
	cfg.Layout = params.layout
	cfg.ClusterPrefix = "fake-test"

	return cfg
}

type testProvideMetaConfigParams struct {
	env, testName, layout, uuid string
	logger                      log.Logger
}

func provideTestMetaConfig(t *testing.T, params testProvideMetaConfigParams) *config.MetaConfig {
	require.NotEmpty(t, params.env)
	require.NotEmpty(t, params.testName)
	require.NotEmpty(t, params.layout)
	require.NotEmpty(t, params.uuid)
	require.False(t, govalue.IsNil(params.logger))

	configPath := os.Getenv(params.env)

	if configPath == "" {
		t.Skip(fmt.Sprintf("Skipping %s test. Use %s for provide configuration", params.testName, params.env))
	}

	stat, err := os.Stat(configPath)
	require.NoError(t, err)
	require.False(t, stat.IsDir())

	cfg, err := config.ParseConfig(context.TODO(), []string{configPath}, MetaConfigPreparatorProvider(PreparatorProviderParams{
		logger: params.logger,
	}))

	require.NoError(t, err)
	require.Equal(t, params.layout, cfg.Layout, "layout should be", params.layout)

	cfg.UUID = params.uuid

	return cfg
}

func assertLockFilePresent(t *testing.T, root string) {
	t.Helper()

	stat, err := os.Stat(path.Join(root, lockFile))
	require.NoError(t, err)
	require.False(t, stat.IsDir())
}

type stringOrRegex struct {
	regex    *regexp.Regexp
	value    string
	excludes []string
}

func assertFileOrDirDoesNotPresentsInDir(t *testing.T, dir string, file stringOrRegex) {
	t.Helper()

	if file.regex == nil {
		require.NotEmpty(t, file.value)
	}

	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		require.NoError(t, err, p)

		filename := path.Base(p)
		require.NotEqual(t, filename, "/", p)

		if len(file.excludes) > 0 {
			if slices.Contains(file.excludes, filename) {
				fmt.Printf("Found full match exclude %s\n", p)
				return nil
			}
		}

		if file.regex != nil {
			require.False(t, file.regex.MatchString(filename), p)
			return nil
		}

		require.False(t, strings.HasPrefix(filename, file.value), p)
		return nil
	})

	require.NoError(t, err)
}

func assertDirsNotContainsFileInFSSources(t *testing.T, params CloudProviderGetterParams, file stringOrRegex) {
	t.Helper()

	require.NotNil(t, params.FSDIParams)

	assertFileOrDirDoesNotPresentsInDir(t, params.FSDIParams.BinariesDir, file)
	assertFileOrDirDoesNotPresentsInDir(t, params.FSDIParams.CloudProviderDir, file)
	assertFileOrDirDoesNotPresentsInDir(t, params.FSDIParams.PluginsDir, file)
}

func assertDirsNotContainsLockFile(t *testing.T, params CloudProviderGetterParams) {
	t.Helper()

	require.NotNil(t, params.FSDIParams)

	assertDirsNotContainsFileInFSSources(t, params, stringOrRegex{value: lockFile})
}

type executorTestInitParams struct {
	provider      infrastructure.CloudProvider
	step          infrastructure.Step
	params        CloudProviderGetterParams
	layout        string
	pluginsDir    []string
	pluginVersion string
}

func asserProviderDirContainsWorkingFilesAndSourcesNotContainsLock(t *testing.T, params executorTestInitParams) {
	t.Helper()

	require.False(t, govalue.IsNil(params.provider))
	require.NotEmpty(t, params.step)
	require.NotEmpty(t, params.layout)
	require.NotEmpty(t, params.pluginsDir)
	require.NotEmpty(t, params.pluginVersion)
	require.NotNil(t, params.params.FSDIParams)

	infraRoot := filepath.Join(params.provider.RootDir(), params.pluginVersion)

	tmp := filepath.Join(infraRoot, "tf_dhctl")

	assertIsNotEmptyDir(t, tmp)
	assertPluginsPresent(t, path.Join(tmp, "providers"), params.pluginsDir, params.params.FSDIParams.PluginsDir)
	assertFileExists(t, path.Join(tmp, "plugin_path"))

	lockFileDir := infraRoot
	if params.provider.NeedToUseTofu() {
		lockFileDir = getTestStepDir(infraRoot, params.step, params.layout)
	}

	assertLockFilePresent(t, lockFileDir)
	assertDirsNotContainsLockFile(t, params.params)
	assertDirsNotContainsFileInFSSources(t, params.params, stringOrRegex{value: "lock.json"})
}

func assertPlanResult(t *testing.T, planParams infrastructure.PlanOpts, exitCode int, params CloudProviderGetterParams, err error) {
	t.Helper()

	require.NotNil(t, params.FSDIParams)

	assertDirsNotContainsLockFile(t, params)

	require.NotEqual(t, exitCode, 0)
	require.Error(t, err)
	assertFileExists(t, planParams.VariablesPath)
	assertFileExists(t, planParams.StatePath)
	assertFileExistsAndHasAnyContent(t, planParams.OutPath)

	assertDirsNotContainsFileInFSSources(t, params, stringOrRegex{
		value: path.Base(planParams.OutPath),
	})
	assertDirsNotContainsFileInFSSources(t, params, stringOrRegex{
		value: path.Base(planParams.VariablesPath),
	})
	assertDirsNotContainsFileInFSSources(t, params, stringOrRegex{
		value: path.Base(planParams.StatePath),
	})
}

type getTestParamsPlanParams struct {
	provider       infrastructure.CloudProvider
	configProvider func() []byte
	step           infrastructure.Step
	pluginVersion  string
	layout         string
}

func getTestPlanParams(t *testing.T, params getTestParamsPlanParams) infrastructure.PlanOpts {
	t.Helper()

	require.False(t, govalue.IsNil(params.provider))
	require.NotNil(t, params.configProvider)
	require.NotEmpty(t, params.step)
	require.NotEmpty(t, params.pluginVersion)
	require.NotEmpty(t, params.layout)

	infraRoot := filepath.Join(params.provider.RootDir(), params.pluginVersion)

	planParams := infrastructure.PlanOpts{
		Destroy:          false,
		StatePath:        filepath.Join(infraRoot, fmt.Sprintf("state__%s_%s.tfstate", params.layout, params.step)),
		VariablesPath:    filepath.Join(infraRoot, fmt.Sprintf("variables_%s_%s.tfvars.json", params.layout, params.step)),
		OutPath:          filepath.Join(infraRoot, fmt.Sprintf("output_%s_%s.tfplan", params.layout, params.step)),
		DetailedExitCode: true,
	}

	content := params.configProvider()
	err := os.WriteFile(planParams.VariablesPath, content, 0o666)
	require.NoError(t, err)

	err = os.WriteFile(planParams.StatePath, nil, 0o666)
	require.NoError(t, err)

	return planParams
}

type execInitAndPlanResultsParams struct {
	provider       infrastructure.CloudProvider
	step           infrastructure.Step
	params         CloudProviderGetterParams
	configProvider func() []byte
	layout         string
	pluginsDir     []string
	pluginVersion  string
}

func assertTerraformDirNotExistsInHomeAndPresentInInfraRoot(t *testing.T, params execInitAndPlanResultsParams) {
	t.Helper()

	require.False(t, govalue.IsNil(params.provider))
	require.NotEmpty(t, params.pluginVersion)
	require.NotEmpty(t, params.pluginsDir)
	require.NotNil(t, params.params.FSDIParams)

	const terraformDir = ".terraform.d"

	if os.Getenv("SKIP_TEST_PROVIDER_TERRAFORM_HOME") == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		full := filepath.Join(home, terraformDir)
		assertDirNotExists(t, full, fmt.Sprintf("Dir %s present for skip use SKIP_TEST_PROVIDER_TERRAFORM_HOME=true env", full))
	} else {
		params.params.Logger.LogInfoF("%s dir in home was skipped\n", terraformDir)
	}

	infraRoot := filepath.Join(params.provider.RootDir(), params.pluginVersion)
	assertDirExists(t, filepath.Join(infraRoot, terraformDir))

	getExcludes := func(params execInitAndPlanResultsParams, infraBin string) []string {
		excludes := []string{
			"terraform-modules",
			"registry.terraform.io",
			"registry.opentofu.org",
			infraBin,
		}

		for _, p := range yandexPluginsDir {
			excludes = append(excludes, filepath.Base(p))
		}

		for _, p := range gcpPluginsDir {
			excludes = append(excludes, filepath.Base(p))
		}

		return excludes
	}

	assertDirsNotContainsFileInFSSources(t, params.params, stringOrRegex{
		regex:    regexp.MustCompile(".*terraform.*"),
		excludes: getExcludes(params, terraformBin),
	})

	assertDirsNotContainsFileInFSSources(t, params.params, stringOrRegex{
		regex:    regexp.MustCompile(".*tofu.*"),
		excludes: getExcludes(params, tofuBin),
	})
}

func assertExecInitAndPlanResults(t *testing.T, params execInitAndPlanResultsParams) (infrastructure.Executor, infrastructure.PlanOpts) {
	t.Helper()

	require.False(t, govalue.IsNil(params.provider))
	require.NotNil(t, params.configProvider)
	require.NotEmpty(t, params.step)
	require.NotEmpty(t, params.layout)
	require.NotEmpty(t, params.pluginVersion)
	require.NotNil(t, params.pluginsDir)

	executor, err := params.provider.Executor(context.TODO(), params.step, params.params.Logger)
	require.NoError(t, err)

	err = executor.Init(context.TODO())
	require.NoError(t, err)
	asserProviderDirContainsWorkingFilesAndSourcesNotContainsLock(t, executorTestInitParams{
		provider:      params.provider,
		step:          params.step,
		params:        params.params,
		layout:        params.layout,
		pluginsDir:    params.pluginsDir,
		pluginVersion: params.pluginVersion,
	})

	assertTerraformDirNotExistsInHomeAndPresentInInfraRoot(t, params)

	planParams := getTestPlanParams(t, getTestParamsPlanParams{
		provider:       params.provider,
		step:           params.step,
		configProvider: params.configProvider,
		pluginVersion:  params.pluginVersion,
		layout:         params.layout,
	})

	exitCode, err := executor.Plan(context.TODO(), planParams)
	assertPlanResult(t, planParams, exitCode, params.params, err)
	assertTerraformDirNotExistsInHomeAndPresentInInfraRoot(t, params)

	return executor, planParams
}
