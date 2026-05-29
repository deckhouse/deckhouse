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

	"github.com/Masterminds/semver/v3"
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

// TestLoadAppConfModulesRequirements tests that mandatory, conditional, anyOf,
// and noneOf module dependencies in an application's package.yaml are parsed
// into the respective shapes, that constraint strings are honored, that
// mandatory entries may omit the constraint (parsed as a nil *semver.Constraints
// meaning "any version"), and that group buckets carry their Name plus
// per-member constraints with the right empty-constraint semantics per bucket.
func (s *LoaderTestSuite) TestLoadAppConfModulesRequirements() {
	packageDir := filepath.Join(s.testdataDir, "apps", "default.complete-app")

	cfg, err := loader.LoadAppConf(context.Background(), packageDir, s.logger)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	mandatory := cfg.Definition.Requirements.Modules.Mandatory
	conditional := cfg.Definition.Requirements.Modules.Conditional
	anyOf := cfg.Definition.Requirements.Modules.AnyOf
	noneOf := cfg.Definition.Requirements.Modules.NoneOf

	require.Len(s.T(), mandatory, 2)
	require.Len(s.T(), conditional, 1)
	require.Len(s.T(), anyOf, 1)
	require.Len(s.T(), noneOf, 1)

	// Mandatory with constraint: constraint accepts >=1.14, rejects 1.13.
	cniConstraint, ok := mandatory["cni-cilium"]
	require.True(s.T(), ok, "cni-cilium must be in mandatory map")
	require.NotNil(s.T(), cniConstraint)
	s.True(cniConstraint.Check(semver.MustParse("1.14.0")))
	s.False(cniConstraint.Check(semver.MustParse("1.13.0")))

	// Mandatory without constraint: present in map with nil constraint (any version).
	regConstraint, ok := mandatory["registry-packages-proxy"]
	require.True(s.T(), ok, "registry-packages-proxy must be in mandatory map")
	s.Nil(regConstraint)

	// Conditional with constraint: accepts >=2.40, rejects 2.39.
	promConstraint, ok := conditional["prometheus"]
	require.True(s.T(), ok, "prometheus must be in conditional map")
	require.NotNil(s.T(), promConstraint)
	s.True(promConstraint.Check(semver.MustParse("2.40.0")))
	s.False(promConstraint.Check(semver.MustParse("2.39.0")))

	// AnyOf group: name carried through; both members present with the parsed
	// constraints. Group is checker-only at the scheduler layer, but at the
	// loader boundary we just verify the parse fidelity.
	group := anyOf[0]
	s.Equal("cloud-provider", group.Name)
	require.Len(s.T(), group.Members, 2)

	gcpConstraint, ok := group.Members["cloud-provider-gcp"]
	require.True(s.T(), ok, "cloud-provider-gcp must be in anyOf group members")
	require.NotNil(s.T(), gcpConstraint)
	s.True(gcpConstraint.Check(semver.MustParse("1.5.0")))
	s.False(gcpConstraint.Check(semver.MustParse("1.4.0")))

	awsConstraint, ok := group.Members["cloud-provider-aws"]
	require.True(s.T(), ok, "cloud-provider-aws must be in anyOf group members")
	require.NotNil(s.T(), awsConstraint)
	s.True(awsConstraint.Check(semver.MustParse("2.0.0")))
	s.False(awsConstraint.Check(semver.MustParse("1.9.0")))

	// NoneOf group: forbidden modules. nginx-ingress-legacy carries a non-nil
	// constraint scoping the forbidden range to <2.0.0 (so 2.0.0+ is fine).
	// haproxy-legacy has a nil constraint meaning "forbidden at any version".
	noneOfGroup := noneOf[0]
	s.Equal("legacy-ingress", noneOfGroup.Name)
	require.Len(s.T(), noneOfGroup.Members, 2)

	nginxConstraint, ok := noneOfGroup.Members["nginx-ingress-legacy"]
	require.True(s.T(), ok, "nginx-ingress-legacy must be in noneOf group members")
	require.NotNil(s.T(), nginxConstraint)
	s.True(nginxConstraint.Check(semver.MustParse("1.9.0")), "1.9.0 is in the forbidden range")
	s.False(nginxConstraint.Check(semver.MustParse("2.0.0")), "2.0.0 is outside the forbidden range")

	haproxyConstraint, ok := noneOfGroup.Members["haproxy-legacy"]
	require.True(s.T(), ok, "haproxy-legacy must be in noneOf group members")
	s.Nil(haproxyConstraint, "nil constraint means forbidden at any installed version")
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

	// Verify static values
	s.NotNil(cfg.StaticValues)
	s.Equal(true, cfg.StaticValues["enabled"])
	s.Equal("info", cfg.StaticValues["logLevel"])

	// Verify OpenAPI schema loaded
	s.NotNil(cfg.ConfigSchema)
	s.Contains(string(cfg.ConfigSchema), "type: object")
}

// TestLoadModuleConfModulesRequirements tests that mandatory, conditional, anyOf,
// and noneOf module dependencies in a module's package.yaml are parsed into the
// respective shapes, that constraint strings are honored, that mandatory entries
// may omit the constraint (parsed as a nil *semver.Constraints meaning "any
// version"), and that group buckets carry their Name plus per-member constraints
// with the right empty-constraint semantics per bucket.
func (s *LoaderTestSuite) TestLoadModuleConfModulesRequirements() {
	packageDir := filepath.Join(s.testdataDir, "modules", "complete-module")

	cfg, err := loader.LoadModuleConf(context.Background(), packageDir, s.logger)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), cfg)

	mandatory := cfg.Definition.Requirements.Modules.Mandatory
	conditional := cfg.Definition.Requirements.Modules.Conditional
	anyOf := cfg.Definition.Requirements.Modules.AnyOf
	noneOf := cfg.Definition.Requirements.Modules.NoneOf

	require.Len(s.T(), mandatory, 2)
	require.Len(s.T(), conditional, 1)
	require.Len(s.T(), anyOf, 1)
	require.Len(s.T(), noneOf, 1)

	// Mandatory with constraint: constraint accepts >=1.14, rejects 1.13.
	cniConstraint, ok := mandatory["cni-cilium"]
	require.True(s.T(), ok, "cni-cilium must be in mandatory map")
	require.NotNil(s.T(), cniConstraint)
	s.True(cniConstraint.Check(semver.MustParse("1.14.0")))
	s.False(cniConstraint.Check(semver.MustParse("1.13.0")))

	// Mandatory without constraint: present in map with nil constraint (any version).
	regConstraint, ok := mandatory["registry-packages-proxy"]
	require.True(s.T(), ok, "registry-packages-proxy must be in mandatory map")
	s.Nil(regConstraint)

	// Conditional with constraint: accepts >=2.40, rejects 2.39.
	promConstraint, ok := conditional["prometheus"]
	require.True(s.T(), ok, "prometheus must be in conditional map")
	require.NotNil(s.T(), promConstraint)
	s.True(promConstraint.Check(semver.MustParse("2.40.0")))
	s.False(promConstraint.Check(semver.MustParse("2.39.0")))

	// AnyOf group: name carried through; both members present with the parsed
	// constraints. Group is checker-only at the scheduler layer, but at the
	// loader boundary we just verify the parse fidelity.
	group := anyOf[0]
	s.Equal("cloud-provider", group.Name)
	require.Len(s.T(), group.Members, 2)

	gcpConstraint, ok := group.Members["cloud-provider-gcp"]
	require.True(s.T(), ok, "cloud-provider-gcp must be in anyOf group members")
	require.NotNil(s.T(), gcpConstraint)
	s.True(gcpConstraint.Check(semver.MustParse("1.5.0")))
	s.False(gcpConstraint.Check(semver.MustParse("1.4.0")))

	awsConstraint, ok := group.Members["cloud-provider-aws"]
	require.True(s.T(), ok, "cloud-provider-aws must be in anyOf group members")
	require.NotNil(s.T(), awsConstraint)
	s.True(awsConstraint.Check(semver.MustParse("2.0.0")))
	s.False(awsConstraint.Check(semver.MustParse("1.9.0")))

	// NoneOf group: forbidden modules. nginx-ingress-legacy carries a non-nil
	// constraint scoping the forbidden range to <2.0.0 (so 2.0.0+ is fine).
	// haproxy-legacy has a nil constraint meaning "forbidden at any version".
	noneOfGroup := noneOf[0]
	s.Equal("legacy-ingress", noneOfGroup.Name)
	require.Len(s.T(), noneOfGroup.Members, 2)

	nginxConstraint, ok := noneOfGroup.Members["nginx-ingress-legacy"]
	require.True(s.T(), ok, "nginx-ingress-legacy must be in noneOf group members")
	require.NotNil(s.T(), nginxConstraint)
	s.True(nginxConstraint.Check(semver.MustParse("1.9.0")), "1.9.0 is in the forbidden range")
	s.False(nginxConstraint.Check(semver.MustParse("2.0.0")), "2.0.0 is outside the forbidden range")

	haproxyConstraint, ok := noneOfGroup.Members["haproxy-legacy"]
	require.True(s.T(), ok, "haproxy-legacy must be in noneOf group members")
	s.Nil(haproxyConstraint, "nil constraint means forbidden at any installed version")
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
