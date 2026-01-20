// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
)

func TestEditionBad(t *testing.T) {
	origEdition, origVersion := app.AppEdition, app.AppVersion
	t.Cleanup(func() {
		app.AppEdition = origEdition
		app.AppVersion = origVersion
	})

	app.AppVersion = "dev"
	app.AppEdition = "test"
	t.Setenv("DHCTL_TEST_VERSION_TAG", "v1.2.3")

	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo("test.registry.io/test"),
		registry_mocks.WithSchemeHTTPS(),
	)

	installer := &config.DeckhouseInstaller{
		Registry:  registryCfg,
		DevBranch: "dev-branch",
	}
	metaCfg := &config.MetaConfig{
		Registry: registryCfg,
	}

	image := installer.GetRemoteImage(true)
	ref, err := name.ParseReference(image)
	require.NoError(t, err)

	provider := NewFakeImageDescriptorProvider(t).
		ExpectReference(ref).
		Return(&v1.ConfigFile{
			Config: v1.Config{Labels: map[string]string{
				"io.deckhouse.edition": "BAD",
			}},
		}, nil)

	check := DhctlEditionCheck{
		MetaConfig: metaCfg,
		Installer:  installer,
		descriptor: provider,
	}

	err = check.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestOk(t *testing.T) {
	origEdition, origVersion := app.AppEdition, app.AppVersion
	t.Cleanup(func() {
		app.AppEdition = origEdition
		app.AppVersion = origVersion
	})

	app.AppVersion = "dev"
	app.AppEdition = "test"
	t.Setenv("DHCTL_TEST_VERSION_TAG", "v1.2.3")

	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo("test.registry.io/test"),
		registry_mocks.WithSchemeHTTPS(),
	)

	installer := &config.DeckhouseInstaller{
		Registry:  registryCfg,
		DevBranch: "dev-branch",
	}
	metaCfg := &config.MetaConfig{
		Registry: registryCfg,
	}

	image := installer.GetRemoteImage(true)
	ref, err := name.ParseReference(image)
	require.NoError(t, err)

	provider := NewFakeImageDescriptorProvider(t).
		ExpectReference(ref).
		Return(&v1.ConfigFile{
			Config: v1.Config{Labels: map[string]string{
				"io.deckhouse.edition": "test",
			}},
		}, nil)

	check := DhctlEditionCheck{
		MetaConfig: metaCfg,
		Installer:  installer,
		descriptor: provider,
	}

	err = check.Run(context.Background())
	assert.NoError(t, err)
}

func TestCheckDisable(t *testing.T) {
	origEdition, origVersion := app.AppEdition, app.AppVersion
	t.Cleanup(func() {
		app.AppEdition = origEdition
		app.AppVersion = origVersion
	})

	app.AppVersion = "local"
	app.AppEdition = "local"

	assert.False(t, DhctlEditionCheck{}.Enabled())
}
