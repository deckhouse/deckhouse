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

package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const testdataValuesDir = "testdata/values"

// ValuesTestSuite tests values loading functionality.
type ValuesTestSuite struct {
	suite.Suite
	testdataDir string
	tempDir     string
}

func TestValuesTestSuite(t *testing.T) {
	suite.Run(t, new(ValuesTestSuite))
}

func (s *ValuesTestSuite) SetupSuite() {
	cwd, err := os.Getwd()
	require.NoError(s.T(), err)
	s.testdataDir = filepath.Join(cwd, testdataValuesDir)
}

func (s *ValuesTestSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "loader-test-*")
	require.NoError(s.T(), err)
}

func (s *ValuesTestSuite) TearDownTest() {
	os.RemoveAll(s.tempDir)
}

// TestLoadValuesCompletePackage tests loading values from a package with all files present.
func (s *ValuesTestSuite) TestLoadValuesCompletePackage() {
	packageDir := filepath.Join(s.testdataDir, "complete-package")

	static, config, values, err := loadValues("complete-package", packageDir)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), static)
	require.NotNil(s.T(), config)
	require.NotNil(s.T(), values)

	// Verify static values are loaded correctly
	s.Equal(float64(3), static["replicas"])
	s.Equal("nginx:latest", static["image"])

	// Verify resources nested structure
	resources, ok := static["resources"].(map[string]any)
	require.True(s.T(), ok)
	limits, ok := resources["limits"].(map[string]any)
	require.True(s.T(), ok)
	s.Equal("100m", limits["cpu"])
	s.Equal("128Mi", limits["memory"])

	// Verify OpenAPI schemas are loaded as raw bytes
	s.Contains(string(config), "type: object")
	s.Contains(string(config), "replicas")
	s.Contains(string(values), "type: object")
	s.Contains(string(values), "resources")
}

// TestLoadValuesScopedValues tests that values scoped under package name are extracted.
func (s *ValuesTestSuite) TestLoadValuesScopedValues() {
	packageDir := filepath.Join(s.testdataDir, "scoped-values")

	// Package name "my-app" converts to "myApp" for values key
	static, config, _, err := loadValues("my-app", packageDir)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), static)
	require.NotNil(s.T(), config)

	// Values should be extracted from under the "myApp" key
	s.Equal(true, static["enabled"])
	s.Equal(float64(8080), static["port"])

	// Verify nested settings
	settings, ok := static["settings"].(map[string]any)
	require.True(s.T(), ok)
	s.Equal(false, settings["debug"])
}

// TestLoadValuesNoOpenAPI tests loading values when OpenAPI schemas are missing.
func (s *ValuesTestSuite) TestLoadValuesNoOpenAPI() {
	packageDir := filepath.Join(s.testdataDir, "no-openapi")

	static, config, values, err := loadValues("no-openapi", packageDir)

	require.NoError(s.T(), err)
	require.NotNil(s.T(), static)

	// OpenAPI schemas should be nil when files don't exist
	s.Nil(config)
	s.Nil(values)

	// Static values should still be loaded
	s.Equal("value", static["simple"])
	s.Equal(float64(42), static["count"])
}

// TestLoadValuesEmptyPackage tests loading values from an empty package directory.
func (s *ValuesTestSuite) TestLoadValuesEmptyPackage() {
	packageDir := filepath.Join(s.testdataDir, "empty-package")

	static, config, values, err := loadValues("empty-package", packageDir)

	require.NoError(s.T(), err)

	// All should be empty/nil for empty package
	s.Empty(static)
	s.Nil(config)
	s.Nil(values)
}

// TestLoadValuesNonExistentPackage tests error handling for non-existent package.
func (s *ValuesTestSuite) TestLoadValuesNonExistentPackage() {
	packageDir := filepath.Join(s.testdataDir, "non-existent")

	static, config, values, err := loadValues("non-existent", packageDir)

	// Should not error - empty values are valid
	require.NoError(s.T(), err)
	s.Empty(static)
	s.Nil(config)
	s.Nil(values)
}

// TestLoadPackageSchemasBothFiles tests loading when both schema files exist.
func (s *ValuesTestSuite) TestLoadPackageSchemasBothFiles() {
	openapiDir := filepath.Join(s.tempDir, openAPIDir)
	require.NoError(s.T(), os.MkdirAll(openapiDir, 0755))

	configContent := []byte("config: schema")
	valuesContent := []byte("values: schema")
	require.NoError(s.T(), os.WriteFile(filepath.Join(openapiDir, configValuesFile), configContent, 0644))
	require.NoError(s.T(), os.WriteFile(filepath.Join(openapiDir, valuesFile), valuesContent, 0644))

	config, values, err := loadPackageSchemas(s.tempDir)

	require.NoError(s.T(), err)
	s.Equal(configContent, config)
	s.Equal(valuesContent, values)
}

// TestLoadPackageSchemasOnlyConfigValues tests loading when only config-values.yaml exists.
func (s *ValuesTestSuite) TestLoadPackageSchemasOnlyConfigValues() {
	openapiDir := filepath.Join(s.tempDir, openAPIDir)
	require.NoError(s.T(), os.MkdirAll(openapiDir, 0755))

	configContent := []byte("config: only")
	require.NoError(s.T(), os.WriteFile(filepath.Join(openapiDir, configValuesFile), configContent, 0644))

	config, values, err := loadPackageSchemas(s.tempDir)

	require.NoError(s.T(), err)
	s.Equal(configContent, config)
	s.Nil(values)
}

// TestLoadPackageSchemasOnlyValues tests loading when only values.yaml exists.
func (s *ValuesTestSuite) TestLoadPackageSchemasOnlyValues() {
	openapiDir := filepath.Join(s.tempDir, openAPIDir)
	require.NoError(s.T(), os.MkdirAll(openapiDir, 0755))

	valuesContent := []byte("values: only")
	require.NoError(s.T(), os.WriteFile(filepath.Join(openapiDir, valuesFile), valuesContent, 0644))

	config, values, err := loadPackageSchemas(s.tempDir)

	require.NoError(s.T(), err)
	s.Nil(config)
	s.Equal(valuesContent, values)
}

// TestLoadPackageSchemasEmptyOpenAPIDir tests loading when openapi directory is empty.
func (s *ValuesTestSuite) TestLoadPackageSchemasEmptyOpenAPIDir() {
	openapiDir := filepath.Join(s.tempDir, openAPIDir)
	require.NoError(s.T(), os.MkdirAll(openapiDir, 0755))

	config, values, err := loadPackageSchemas(s.tempDir)

	require.NoError(s.T(), err)
	s.Nil(config)
	s.Nil(values)
}

// TestLoadPackageSchemasNoOpenAPIDir tests loading when openapi directory doesn't exist.
func (s *ValuesTestSuite) TestLoadPackageSchemasNoOpenAPIDir() {
	config, values, err := loadPackageSchemas(s.tempDir)

	require.NoError(s.T(), err)
	s.Nil(config)
	s.Nil(values)
}

// TestLoadPackageSchemasEmptyFiles tests loading when schema files exist but are empty.
func (s *ValuesTestSuite) TestLoadPackageSchemasEmptyFiles() {
	openapiDir := filepath.Join(s.tempDir, openAPIDir)
	require.NoError(s.T(), os.MkdirAll(openapiDir, 0755))

	require.NoError(s.T(), os.WriteFile(filepath.Join(openapiDir, configValuesFile), []byte{}, 0644))
	require.NoError(s.T(), os.WriteFile(filepath.Join(openapiDir, valuesFile), []byte{}, 0644))

	config, values, err := loadPackageSchemas(s.tempDir)

	require.NoError(s.T(), err)
	s.Empty(config)
	s.Empty(values)
}
