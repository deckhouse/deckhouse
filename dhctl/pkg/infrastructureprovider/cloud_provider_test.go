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
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/interfaces"
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
)

var (
	yandexPluginsDir = []string{
		fmt.Sprintf("registry.opentofu.org/yandex-cloud/yandex/%s/linux_amd64/terraform-provider-yandex", yandexPluginVersion),
	}
	gcpPluginsDir = []string{
		fmt.Sprintf("registry.terraform.io/hashicorp/google/%s/linux_amd64/terraform-provider-google", gcpPluginVersion),
	}
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

	assertCloudProvider(t, providerYandex, yandex.ProviderName, true)
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

	assertCloudProvider(t, providerGCP, gcp.ProviderName, false)
	require.True(t, strings.HasSuffix(providerGCP.RootDir(), "infra/72ce5a172c9b8efa"))
	require.NotEqual(t, providerGCP.RootDir(), providerYandex.RootDir())
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

	_, err = providerYandex.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)

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

	assertCleanupNotFaultAndKeepFSDIDirsAndFiles(t, providerYandex, params)
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

	_, err = providerGCP.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)

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

	// does not corrupt is another step used
	anotherStep := infrastructure.MasterNodeStep
	testParams.usedStep = anotherStep
	_, err = providerGCP.Executor(context.TODO(), step, params.Logger)
	require.NoError(t, err)
	assertAllFilesCopiedToProviderDir(t, testParams, params)

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

	assertCleanupNotFaultAndKeepFSDIDirsAndFiles(t, providerGCP, params)
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

func assertCloudProvider(t *testing.T, provider infrastructure.CloudProvider, providerName string, useTofu bool) {
	t.Helper()

	require.False(t, interfaces.IsNil(provider))
	require.IsType(t, &cloud.Provider{}, provider, "provider should be a cloud.Provider for", providerName)
	require.Equal(t, provider.NeedToUseTofu(), useTofu)
	require.Equal(t, provider.Name(), providerName)
}

func assertFileExistsAndSymlink(t *testing.T, source string, destination string) {
	t.Helper()

	stat, err := os.Lstat(destination)
	require.NoError(t, err, destination)
	require.True(t, stat.Mode()&os.ModeSymlink != 0, destination)

	realPath, err := os.Readlink(destination)
	require.NoError(t, err, destination)
	require.Equal(t, source, realPath)
}

func assertFileExists(t *testing.T, filePath string) {
	t.Helper()

	stat, err := os.Stat(filePath)
	require.NoError(t, err, filePath)

	require.True(t, stat.Mode()&os.ModeSymlink == 0, filePath)
	require.False(t, stat.IsDir(), filePath)
}

func assertFileExistsAndHasAnyContent(t *testing.T, filePath string) {
	t.Helper()

	assertFileExists(t, filePath)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, filePath)
	require.True(t, len(content) > 0, filePath)
}

func assertFilExistsAndHasContent(t *testing.T, filePath string, expectedContent string) {
	t.Helper()

	assertFileExists(t, filePath)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err, filePath)

	require.Equal(t, expectedContent, string(content), filePath)
}

func assertIsNotEmptyDir(t *testing.T, dirPath string) {
	t.Helper()

	stat, err := os.Stat(dirPath)
	require.NoError(t, err, dirPath)
	require.True(t, stat.IsDir(), dirPath)

	entries, err := os.ReadDir(dirPath)
	require.NoError(t, err, dirPath)
	require.True(t, len(entries) > 0, dirPath)
}

func assertDirNotExists(t *testing.T, dirPath string) {
	t.Helper()

	_, err := os.Stat(dirPath)
	require.True(t, os.IsNotExist(err), dirPath)
}

func assertFSDIDirsAndFilesExists(t *testing.T, params CloudProviderGetterParams) {
	require.NotNil(t, params.FSDIParams)

	assertIsNotEmptyDir(t, params.FSDIParams.PluginsDir)
	assertIsNotEmptyDir(t, params.FSDIParams.BinariesDir)
	assertIsNotEmptyDir(t, params.FSDIParams.CloudProviderDir)
	assertFileExistsAndHasAnyContent(t, params.FSDIParams.InfraVersionsFile)
}

func assertCleanupNotFaultAndKeepFSDIDirsAndFiles(t *testing.T, provider infrastructure.CloudProvider, params CloudProviderGetterParams) {
	require.False(t, interfaces.IsNil(provider))
	require.NotNil(t, params.FSDIParams)

	// cleanup
	err := provider.Cleanup()
	require.NoError(t, err)
	assertDirNotExists(t, provider.RootDir())
	assertFSDIDirsAndFilesExists(t, params)

	// double cleanup does not provide error
	err = provider.Cleanup()
	require.NoError(t, err)
	assertDirNotExists(t, provider.RootDir())
	assertFSDIDirsAndFilesExists(t, params)
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

func assertInfraUtilCopied(t *testing.T, provider infrastructure.CloudProvider, providerParams CloudProviderGetterParams, pluginVersion string) {
	t.Helper()

	infraBin := "terraform"
	if provider.NeedToUseTofu() {
		infraBin = "opentofu"
	}

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
	require.False(t, interfaces.IsNil(provider))

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
	assertFilExistsAndHasContent(t, versionsFileWithContentPath, params.versionsContent)

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

	require.False(t, interfaces.IsNil(provider))

	assertInfraUtilCopied(t, provider, providerParams, "")

	entries, err := os.ReadDir(provider.RootDir())
	require.NoError(t, err)
	require.Len(t, entries, 1)
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
	require.False(t, interfaces.IsNil(params.logger))

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

func assertFileDoesNotPresentsInDir(t *testing.T, dir string, file string) {
	t.Helper()

	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		require.NoError(t, err, p)
		if info.IsDir() {
			return nil
		}

		filename := path.Base(p)
		require.NotEqual(t, filename, "/", p)

		require.False(t, strings.HasPrefix(filename, file), p)

		return nil
	})

	require.NoError(t, err)
}

func assertDirsNotContainsFileInFSSources(t *testing.T, params CloudProviderGetterParams, file string) {
	t.Helper()

	require.NotNil(t, params.FSDIParams)

	assertFileDoesNotPresentsInDir(t, params.FSDIParams.BinariesDir, file)
	assertFileDoesNotPresentsInDir(t, params.FSDIParams.CloudProviderDir, file)
	assertFileDoesNotPresentsInDir(t, params.FSDIParams.PluginsDir, file)
}

func assertDirsNotContainsLockFile(t *testing.T, params CloudProviderGetterParams) {
	t.Helper()

	require.NotNil(t, params.FSDIParams)

	assertDirsNotContainsFileInFSSources(t, params, lockFile)
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

	require.False(t, interfaces.IsNil(params.provider))
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
	assertDirsNotContainsFileInFSSources(t, params.params, "lock.json")
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

	assertDirsNotContainsFileInFSSources(t, params, path.Base(planParams.OutPath))
	assertDirsNotContainsFileInFSSources(t, params, path.Base(planParams.VariablesPath))
	assertDirsNotContainsFileInFSSources(t, params, path.Base(planParams.StatePath))
}

type getTestParamsPlanParams struct {
	provider       infrastructure.CloudProvider
	configProvider func() []byte
	step           infrastructure.Step
	pluginVersion  string
}

func getTestPlanParams(t *testing.T, params getTestParamsPlanParams) infrastructure.PlanOpts {
	t.Helper()

	require.False(t, interfaces.IsNil(params.provider))
	require.NotNil(t, params.configProvider)
	require.NotEmpty(t, params.step)
	require.NotEmpty(t, params.pluginVersion)

	infraRoot := filepath.Join(params.provider.RootDir(), params.pluginVersion)

	planParams := infrastructure.PlanOpts{
		Destroy:          false,
		StatePath:        filepath.Join(infraRoot, fmt.Sprintf("state_%s.tfstate", params.step)),
		VariablesPath:    filepath.Join(infraRoot, fmt.Sprintf("variables_%s.tfvars.json", params.step)),
		OutPath:          filepath.Join(infraRoot, fmt.Sprintf("output_%s.tfplan", params.step)),
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

func assertExecInitAndPlanResults(t *testing.T, params execInitAndPlanResultsParams) {
	t.Helper()

	require.False(t, interfaces.IsNil(params.provider))
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

	planParams := getTestPlanParams(t, getTestParamsPlanParams{
		provider:       params.provider,
		step:           params.step,
		configProvider: params.configProvider,
		pluginVersion:  params.pluginVersion,
	})

	exitCode, err := executor.Plan(context.TODO(), planParams)
	assertPlanResult(t, planParams, exitCode, params.params, err)
}
