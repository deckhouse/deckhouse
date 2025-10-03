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

package fsprovider

import (
	"context"
	"fmt"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/dvp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/gcp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/yandex"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var terraformProviders = []string{
	"openstack",
	"aws",
	gcp.ProviderName,
	"vsphere",
	"azure",
	vcd.ProviderName,
	"huaweicloud",
}

var tofuProviders = []string{
	yandex.ProviderName,
	"dynamix",
	"zvirt",
	dvp.ProviderName,
}

func TestAllProviderPresentInStore(t *testing.T) {
	s, err := loadTerraformVersionFileSettings(infrastructure.GetInfrastructureVersions(), log.GetDefaultLogger())
	require.NoError(t, err)

	all := append(make([]string, 0), tofuProviders...)
	all = append(all, terraformProviders...)

	require.Len(t, s, len(all))
}

func TestProvidersSettings(t *testing.T) {
	s, err := loadTerraformVersionFileSettings(infrastructure.GetInfrastructureVersions(), log.GetDefaultLogger())
	require.NoError(t, err)

	assertSettings := func(t *testing.T, s settingsStore, p string, assertProvider func(t *testing.T, settings settings.ProviderSettings)) {
		require.Contains(t, s, p)
		set := s[p]
		require.NotNil(t, set)

		assertProvider(t, set)

		require.NotEmpty(t, set.CloudName())
		require.NotEmpty(t, set.Namespace())
		require.NotEmpty(t, set.DestinationBinary())
		require.NotEmpty(t, set.Versions())
		require.NotEmpty(t, set.VmResourceType())
	}

	for _, p := range tofuProviders {
		assertSettings(t, s, p, func(t *testing.T, settings settings.ProviderSettings) {
			require.True(t, settings.UseOpenTofu())
			require.Equal(t, settings.InfrastructureVersion(), "1.9.4")
		})
	}

	for _, p := range terraformProviders {
		assertSettings(t, s, p, func(t *testing.T, settings settings.ProviderSettings) {
			require.False(t, settings.UseOpenTofu())
			require.Equal(t, settings.InfrastructureVersion(), "0.14.8")
		})
	}
}

func TestProviderSettingsLoadError(t *testing.T) {
	// settings store returns error on not exists file
	sFailed := newSettingsProvider(log.GetDefaultLogger(), "/not/exists/file-aakjdiejfuefuefjej", func(_ log.Logger, _ string) (settingsStore, error) {
		return nil, fmt.Errorf("file does not exist")
	})
	require.Error(t, sFailed.initError)
	require.Nil(t, sFailed.store)
	require.Len(t, fileToSettingsStore, 0)

	// failed store returns init error due getting
	_, err := sFailed.GetSettings(context.TODO(), yandex.ProviderName, cloud.ProviderAdditionalParams{})
	require.Error(t, err)
}

func TestProviderSettingsLoadedAndStoreInCache(t *testing.T) {
	file := infrastructure.GetInfrastructureVersions()
	logger := log.GetDefaultLogger()

	assertOneStoreInCache := func(t *testing.T, store *SettingsProvider) {
		require.NoError(t, store.initError)
		require.NotNil(t, store)
		require.Len(t, fileToSettingsStore, 1)
		require.Contains(t, fileToSettingsStore, file)
	}

	allProviders := append(make([]string, 0), tofuProviders...)
	allProviders = append(allProviders, terraformProviders...)
	assertGettingDoesNotAffectStores := func(t *testing.T, store *SettingsProvider) {
		require.Len(t, fileToSettingsStore, 1)
		require.Len(t, store.store, len(allProviders))
	}

	sFirst := newSettingsProvider(logger, file, loadOrGetStore)
	assertOneStoreInCache(t, sFirst)

	sSecond := newSettingsProvider(logger, file, loadOrGetStore)
	assertOneStoreInCache(t, sSecond)

	require.Equal(t, sFirst.store, sSecond.store)

	// get settings for existing provider
	settingsYandex, err := sFirst.GetSettings(context.TODO(), yandex.ProviderName, cloud.ProviderAdditionalParams{})
	require.NoError(t, err)
	require.False(t, govalue.IsNil(settingsYandex))
	require.Equal(t, settingsYandex.CloudName(), yandex.ProviderName)
	assertGettingDoesNotAffectStores(t, sFirst)

	// returns error for non exists store
	_, err = sFirst.GetSettings(context.TODO(), "incorrect", cloud.ProviderAdditionalParams{})
	require.Error(t, err)
	assertGettingDoesNotAffectStores(t, sFirst)
}
