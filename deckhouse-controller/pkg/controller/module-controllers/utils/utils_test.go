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

package utils_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestParseDeckhouseRegistrySecret(t *testing.T) {
	t.Run("successfully parses complete data", func(t *testing.T) {
		// First, create the JSON representation
		jsonData := `{
			".dockerconfigjson": "eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlLmNvbSI6eyJhdXRoIjoiZFhObGNqcHdZWE56In19fQ==",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "InRydWUi",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURUVENDQWpXZ0F3SUIKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ=="
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		result, err := utils.ParseDeckhouseRegistrySecret(data)

		require.NoError(t, err)
		assert.Equal(t, `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`, result.DockerConfig)
		assert.Equal(t, "registry.example.com", result.Address)
		assert.True(t, result.ClusterIsBootstrapped)
		assert.Equal(t, "registry.example.com", result.ImageRegistry)
		assert.Equal(t, "/deckhouse", result.Path)
		assert.Equal(t, "https", result.Scheme)
		assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIDTTCCAjWgAwIB\n-----END CERTIFICATE-----", result.CA)
	})

	t.Run("successfully parses data with clusterIsBootstrapped set to false", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlLmNvbSI6eyJhdXRoIjoiZFhObGNqcHdZWE56In19fQ==",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "ZmFsc2U=",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURUVENDQWpXZ0F3SUIKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ=="
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		result, err := utils.ParseDeckhouseRegistrySecret(data)

		require.NoError(t, err)
		assert.Equal(t, `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`, result.DockerConfig)
		assert.Equal(t, "registry.example.com", result.Address)
		assert.False(t, result.ClusterIsBootstrapped)
		assert.Equal(t, "registry.example.com", result.ImageRegistry)
		assert.Equal(t, "/deckhouse", result.Path)
		assert.Equal(t, "https", result.Scheme)
		assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIDTTCCAjWgAwIB\n-----END CERTIFICATE-----", result.CA)
	})

	t.Run("returns error when dockerconfigjson is missing", func(t *testing.T) {
		// First, create the JSON representation
		jsonData := `{
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "dHJ1ZQ==",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrDockerConfigFieldIsNotFound)
	})

	t.Run("returns error when address is missing", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"clusterIsBootstrapped": "dHJ1ZQ==",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrAddressFieldIsNotFound)
	})

	t.Run("returns error when clusterIsBootstrapped is missing", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrClusterIsBootstrappedFieldIsNotFound)
	})

	t.Run("returns error when imagesRegistry is missing", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "dHJ1ZQ==",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrImageRegistryFieldIsNotFound)
	})

	t.Run("returns error when path is missing", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "dHJ1ZQ==",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"scheme": "aHR0cHM=",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrPathFieldIsNotFound)
	})

	t.Run("returns error when scheme is missing", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "dHJ1ZQ==",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrSchemeFieldIsNotFound)
	})

	t.Run("returns error when ca is missing", func(t *testing.T) {
		// First, create the JSON representation with base64-encoded values
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "dHJ1ZQ==",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM="
		}`

		// Then unmarshal it into a map
		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrCAFieldIsNotFound)
	})

	t.Run("returns error when clusterIsBootstrapped has invalid value", func(t *testing.T) {
		jsonData := `{
			".dockerconfigjson": "c29tZS1kb2NrZXItY29uZmln",
			"address": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"clusterIsBootstrapped": "bm90YWJvb2xlYW4=",
			"imagesRegistry": "cmVnaXN0cnkuZXhhbXBsZS5jb20=",
			"path": "L2RlY2tob3VzZQ==",
			"scheme": "aHR0cHM=",
			"ca": "c29tZS1jYS1kYXRh"
		}`

		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "clusterIsBootstrapped is not bool")
	})

	t.Run("returns multiple errors when multiple fields are missing", func(t *testing.T) {
		jsonData := `{
			"clusterIsBootstrapped": "dHJ1ZQ=="
		}`

		data := make(map[string][]byte)
		err := json.Unmarshal([]byte(jsonData), &data)
		require.NoError(t, err)

		_, err = utils.ParseDeckhouseRegistrySecret(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, utils.ErrDockerConfigFieldIsNotFound)
		assert.ErrorIs(t, err, utils.ErrAddressFieldIsNotFound)
		assert.ErrorIs(t, err, utils.ErrImageRegistryFieldIsNotFound)
		assert.ErrorIs(t, err, utils.ErrPathFieldIsNotFound)
		assert.ErrorIs(t, err, utils.ErrSchemeFieldIsNotFound)
		assert.ErrorIs(t, err, utils.ErrCAFieldIsNotFound)
	})
}

func TestGenerateRegistryOptions(t *testing.T) {
	logger := log.NewNop()

	t.Run("with all fields provided", func(t *testing.T) {
		config := &utils.RegistryConfig{
			DockerConfig: `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`,
			CA:           "some-ca-data",
			Scheme:       "https",
			UserAgent:    "test-user-agent",
		}

		options := utils.GenerateRegistryOptions(config, logger)

		assert.Len(t, options, 5)
		// We can't directly check the content of options since the functions are not exported
		// but we can verify they were created
	})

	t.Run("with empty user agent", func(t *testing.T) {
		config := &utils.RegistryConfig{
			DockerConfig: `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`,
			CA:           "some-ca-data",
			Scheme:       "https",
			UserAgent:    "",
		}

		options := utils.GenerateRegistryOptions(config, logger)

		assert.Len(t, options, 5)
		// Default user agent should be set
	})

	t.Run("with HTTP scheme", func(t *testing.T) {
		config := &utils.RegistryConfig{
			DockerConfig: `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`,
			CA:           "some-ca-data",
			Scheme:       "http",
			UserAgent:    "test-user-agent",
		}

		options := utils.GenerateRegistryOptions(config, logger)

		assert.Len(t, options, 5)
		// Insecure schema should be true
	})
}

func TestAvailableModuleSources(t *testing.T) {
	ctx := context.Background()

	sc, err := project.Scheme()
	require.NoError(t, err)

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(
			&v1alpha1.ModulePackage{
				ObjectMeta: metav1.ObjectMeta{Name: "shared"},
				Status:     v1alpha1.ModulePackageStatus{AvailableRepositories: []string{"mirror", "deckhouse-modules"}},
			},
			// the package of an embedded module lists no repository
			&v1alpha1.ModulePackage{ObjectMeta: metav1.ObjectMeta{Name: "embedded-only"}},
		).
		Build()

	shared, err := utils.AvailableModuleSources(ctx, cl, "shared")
	require.NoError(t, err)
	assert.Equal(t, []string{"deckhouse", "mirror"}, shared, "the module sources behind the repositories, sorted")

	embedded, err := utils.AvailableModuleSources(ctx, cl, "embedded-only")
	require.NoError(t, err)
	assert.Empty(t, embedded)

	unknown, err := utils.AvailableModuleSources(ctx, cl, "unknown")
	require.NoError(t, err)
	assert.Nil(t, unknown, "a module without a package is offered by nobody")
}

func TestPickModuleSource(t *testing.T) {
	assert.Empty(t, utils.PickModuleSource("", nil), "nobody offers")
	assert.Equal(t, "mirror", utils.PickModuleSource("", []string{"mirror"}), "the only offering module source")
	assert.Empty(t, utils.PickModuleSource("", []string{"deckhouse", "mirror"}), "several module sources and no choice")
	assert.Equal(t, "mirror", utils.PickModuleSource("mirror", []string{"deckhouse", "mirror"}), "the configured module source wins")
}

func TestHasModuleSourceConflict(t *testing.T) {
	assert.False(t, utils.HasModuleSourceConflict(false, "", []string{"deckhouse", "mirror"}), "a disabled module is never in conflict")
	assert.False(t, utils.HasModuleSourceConflict(true, "mirror", []string{"deckhouse", "mirror"}), "a chosen module source settles the conflict")
	assert.False(t, utils.HasModuleSourceConflict(true, "", []string{"mirror"}), "a single module source is no conflict")
	assert.True(t, utils.HasModuleSourceConflict(true, "", []string{"deckhouse", "mirror"}))
}
