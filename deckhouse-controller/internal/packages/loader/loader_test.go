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

package loader_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// LoaderTestSuite tests application and module loading functionality.
type LoaderTestSuite struct {
	suite.Suite
	testdataDir string
	logger      *log.Logger
}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}

func (s *LoaderTestSuite) SetupSuite() {
	cwd, err := os.Getwd()
	require.NoError(s.T(), err)
	s.testdataDir = filepath.Join(cwd, "testdata")
	s.logger = log.NewNop()
}

// TestLoadAppConfCompletePackage tests loading an application with all files present.
func (s *LoaderTestSuite) TestLoadAppConfCompletePackage() {
	packageDir := filepath.Join(s.testdataDir, "apps", "default.complete-app")

	cfg, err := loader.LoadAppConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	// Verify path
	s.Equal(packageDir, cfg.Path)

	// Verify definition
	s.Equal("complete-app", cfg.Definition.Name)
	s.Equal("v1.0.0", cfg.Definition.Version)

	// Verify requirements
	s.NotNil(cfg.Definition.Requirements.Kubernetes)
	s.NotNil(cfg.Definition.Requirements.Deckhouse)

	// Verify module-dependency buckets resolved from the new YAML shape.
	mods := cfg.Definition.Requirements.Modules
	s.Contains(mods.Mandatory, "prometheus")
	s.Contains(mods.Conditional, "cert-manager")
	s.Require().Len(mods.AnyOf, 1)
	s.Equal("cloud-provider", mods.AnyOf[0].Name)
	s.Contains(mods.AnyOf[0].Modules, "cloud-provider-aws")

	// Verify the subscribe block round-trips through the dto → apps conversion.
	s.ElementsMatch([]string{"apps/v1", "batch/v1"}, cfg.Definition.Subscribe.APIs)
	s.Require().Len(cfg.Definition.Subscribe.Values, 2)
	s.Equal("prometheus", cfg.Definition.Subscribe.Values[0].Module)

	// Verify static values
	s.NotNil(cfg.StaticValues)
	s.Equal(float64(3), cfg.StaticValues["replicas"])
	s.Equal("nginx:latest", cfg.StaticValues["image"])

	// Verify OpenAPI schemas loaded
	s.NotNil(cfg.ConfigSchema)
	s.NotNil(cfg.ValuesSchema)
	s.Contains(string(cfg.ConfigSchema), "type: object")
	s.Contains(string(cfg.ValuesSchema), "type: object")
}

// TestLoadAppConfMinimalPackage tests loading an application with only required files.
func (s *LoaderTestSuite) TestLoadAppConfMinimalPackage() {
	packageDir := filepath.Join(s.testdataDir, "apps", "default.minimal-app")

	cfg, err := loader.LoadAppConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	// Verify definition
	s.Equal("minimal-app", cfg.Definition.Name)
	s.Equal("v0.1.0", cfg.Definition.Version)

	// No requirements set
	s.Nil(cfg.Definition.Requirements.Kubernetes)
	s.Nil(cfg.Definition.Requirements.Deckhouse)

	// No values or schemas
	s.Empty(cfg.StaticValues)
	s.Nil(cfg.ConfigSchema)
	s.Nil(cfg.ValuesSchema)
}

// TestLoadAppConfWithDigests tests loading an application with image digests.
func (s *LoaderTestSuite) TestLoadAppConfWithDigests() {
	packageDir := filepath.Join(s.testdataDir, "apps", "default.with-digests")

	cfg, err := loader.LoadAppConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	// Verify digests loaded
	require.NotNil(s.T(), cfg.Digests)
	s.Equal("sha256:abc123def456", cfg.Digests["nginx"])
	s.Equal("sha256:789xyz000111", cfg.Digests["redis"])
}

// TestLoadAppConfNotFound tests error when package directory doesn't exist.
func (s *LoaderTestSuite) TestLoadAppConfNotFound() {
	packageDir := filepath.Join(s.testdataDir, "apps", "non-existent")

	cfg, err := loader.LoadAppConf(context.Background(), packageDir, s.logger)

	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, loader.ErrPackageNotFound)
	s.Nil(cfg)
}

// TestLoadAppConfInvalidPackageName tests error when package directory name format is invalid.
func (s *LoaderTestSuite) TestLoadAppConfInvalidPackageName() {
	// Create temp dir with invalid name (no dot separator)
	tmpDir := s.T().TempDir()
	packageDir := filepath.Join(tmpDir, "invalid-name-no-dot")
	require.NoError(s.T(), os.MkdirAll(packageDir, 0755))

	// Create valid package.yaml
	content := []byte("name: test\ntype: Application\nversion: v1.0.0\n")
	require.NoError(s.T(), os.WriteFile(filepath.Join(packageDir, "package.yaml"), content, 0644))

	cfg, err := loader.LoadAppConf(context.Background(), packageDir, s.logger)

	require.Error(s.T(), err)
	s.Contains(err.Error(), "invalid package name")
	s.Nil(cfg)
}

// TestLoadModuleConfCompletePackage tests loading a module with all files present.
func (s *LoaderTestSuite) TestLoadModuleConfCompletePackage() {
	packageDir := filepath.Join(s.testdataDir, "modules", "complete-module")

	cfg, err := loader.LoadModuleConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	// Verify path
	s.Equal(packageDir, cfg.Path)

	// Verify definition
	s.Equal("complete-module", cfg.Definition.Name)
	s.Equal("v1.0.0", cfg.Definition.Version)

	// Verify requirements
	s.NotNil(cfg.Definition.Requirements.Kubernetes)
	s.NotNil(cfg.Definition.Requirements.Deckhouse)

	// Verify module-dependency buckets resolved from the new YAML shape.
	mods := cfg.Definition.Requirements.Modules
	s.Contains(mods.Mandatory, "prometheus")
	s.Contains(mods.Conditional, "cert-manager")
	s.Require().Len(mods.AnyOf, 1)
	s.Equal("ingress", mods.AnyOf[0].Name)
	s.Contains(mods.AnyOf[0].Modules, "ingress-nginx")

	// Verify the subscribe block round-trips through the dto → modules conversion.
	s.ElementsMatch([]string{"apps/v1"}, cfg.Definition.Subscribe.APIs)
	s.Require().Len(cfg.Definition.Subscribe.Values, 1)
	s.Equal("prometheus", cfg.Definition.Subscribe.Values[0].Module)

	// Verify static values
	s.NotNil(cfg.StaticValues)
	s.Equal(true, cfg.StaticValues["enabled"])
	s.Equal("info", cfg.StaticValues["logLevel"])

	// Verify OpenAPI schema loaded
	s.NotNil(cfg.ConfigSchema)
	s.Contains(string(cfg.ConfigSchema), "type: object")
}

// TestLoadModuleConfLegacyFallback verifies the package.yaml→module.yaml fallback:
// when a module package directory contains only legacy module.yaml, the loader
// reads it, splits the flat parentModules map into Mandatory / Conditional via
// the "!optional" suffix, and strips any whitespace separator from the
// resulting raw constraint so it parses as clean semver.
func (s *LoaderTestSuite) TestLoadModuleConfLegacyFallback() {
	packageDir := filepath.Join(s.testdataDir, "modules", "legacy-module")

	cfg, err := loader.LoadModuleConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	s.Equal("legacy-module", cfg.Definition.Name)

	mods := cfg.Definition.Requirements.Modules

	// No-suffix entry → Mandatory bucket with the constraint preserved.
	s.Contains(mods.Mandatory, "prometheus")
	s.NotNil(mods.Mandatory["prometheus"], "prometheus constraint must parse as non-nil semver")

	// " !optional" with a space separator → Conditional, raw stripped to ">= 1.10.0".
	// Without TrimSpace the trailing " " would have caused semver.NewConstraint to fail.
	s.Contains(mods.Conditional, "cert-manager")
	s.NotNil(mods.Conditional["cert-manager"], "cert-manager constraint must parse after whitespace trim")

	// "!optional" alone → Conditional with empty constraint (any version satisfies).
	s.Contains(mods.Conditional, "observability")
	s.Nil(mods.Conditional["observability"], "no version constraint should leave the entry nil")

	// Cross-bucket leakage guard.
	s.NotContains(mods.Mandatory, "cert-manager")
	s.NotContains(mods.Mandatory, "observability")
	s.NotContains(mods.Conditional, "prometheus")
}

// TestLoadModuleConfMinimalPackage tests loading a module with only required files.
func (s *LoaderTestSuite) TestLoadModuleConfMinimalPackage() {
	packageDir := filepath.Join(s.testdataDir, "modules", "minimal-module")

	cfg, err := loader.LoadModuleConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	// Verify definition
	s.Equal("minimal-module", cfg.Definition.Name)
	s.Equal("v0.1.0", cfg.Definition.Version)

	// No requirements set
	s.Nil(cfg.Definition.Requirements.Kubernetes)
	s.Nil(cfg.Definition.Requirements.Deckhouse)

	// No values or schemas
	s.Empty(cfg.StaticValues)
	s.Nil(cfg.ConfigSchema)
	s.Nil(cfg.ValuesSchema)
}

// TestLoadModuleConfWithDigests tests loading a module with image digests.
func (s *LoaderTestSuite) TestLoadModuleConfWithDigests() {
	packageDir := filepath.Join(s.testdataDir, "modules", "with-digests")

	cfg, err := loader.LoadModuleConf(context.Background(), packageDir, s.logger)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	// Verify digests loaded
	require.NotNil(s.T(), cfg.Digests)
	s.Equal("sha256:aaa111bbb222", cfg.Digests["controller"])
	s.Equal("sha256:ccc333ddd444", cfg.Digests["webhook"])
}

// TestLoadModuleConfNotFound tests error when package directory doesn't exist.
func (s *LoaderTestSuite) TestLoadModuleConfNotFound() {
	packageDir := filepath.Join(s.testdataDir, "modules", "non-existent")

	cfg, err := loader.LoadModuleConf(context.Background(), packageDir, s.logger)

	require.Error(s.T(), err)
	require.ErrorIs(s.T(), err, loader.ErrPackageNotFound)
	s.Nil(cfg)
}
