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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	opentofuKey  = "opentofu"
	terraformKey = "terraform"
)

type (
	settingsStore map[string]settings.ProviderSettings
	loader        func(ctx context.Context, infraVersionsFile string) (settingsStore, error)
)

type SettingsProvider struct {
	initError error

	m     sync.Mutex
	store settingsStore
}

var (
	fileSettingsStoreMutex sync.Mutex
	fileToSettingsStore    = make(map[string]settingsStore)
)

func loadOrGetStore(ctx context.Context, infraVersionsFile string) (settingsStore, error) {
	fileSettingsStoreMutex.Lock()
	defer fileSettingsStoreMutex.Unlock()

	store, ok := fileToSettingsStore[infraVersionsFile]
	if ok {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Providers settings store for terraform versions file %s loaded from cache", infraVersionsFile))
		return store, nil
	}

	store, err := loadTerraformVersionFileSettings(ctx, infraVersionsFile)
	if err != nil {
		return nil, err
	}

	fileToSettingsStore[infraVersionsFile] = store

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Providers settings store for terraform versions file %s loaded from file and added to cache", infraVersionsFile))

	return store, nil
}

func newSettingsProvider(ctx context.Context, infraVersionsFile string, loader loader) *SettingsProvider {
	store, err := loader(ctx, infraVersionsFile)
	if err != nil {
		return &SettingsProvider{
			initError: err,
		}
	}

	return &SettingsProvider{
		store:     store,
		initError: nil,
	}
}

func (p *SettingsProvider) GetSettings(_ context.Context, provider string, _ cloud.ProviderAdditionalParams) (settings.ProviderSettings, error) {
	if p.initError != nil {
		return nil, p.initError
	}

	p.m.Lock()
	defer p.m.Unlock()

	set, ok := p.store[provider]
	if !ok {
		return nil, fmt.Errorf("CloudProviderSettings not found for provider %s", provider)
	}

	return set, nil
}

func simpleFromMap(s any, terraformVersion, openTofuVersion string) (*settings.Simple, error) {
	sJSON, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	set := settings.Simple{}

	if err := json.Unmarshal(sJSON, &set); err != nil {
		return nil, err
	}

	if err := set.Validate(false); err != nil {
		return nil, err
	}

	if set.UseOpenTofu() {
		set.InfrastructureVersionVal = new(openTofuVersion)
	} else {
		set.InfrastructureVersionVal = new(terraformVersion)
	}

	set.CloudNameVal = new(strings.ToLower(*set.CloudNameVal))

	return &set, nil
}

func loadTerraformVersionFileSettings(ctx context.Context, filename string) (settingsStore, error) {
	infrastructureProviders := make(map[string]any)

	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read infrastructure versions file %s: %v", filename, err)
	}

	err = yaml.Unmarshal(file, &infrastructureProviders)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: %v", filename, err)
	}

	terraformVersion, tofuVersion := "", ""

	for name, rawSettings := range infrastructureProviders {
		var ok bool
		switch name {
		case opentofuKey:
			tofuVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for OpenTofu version setting", name)
			}
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found opentofu version: %s", tofuVersion))
		case terraformKey:
			terraformVersion, ok = rawSettings.(string)
			if !ok {
				return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: wrong type for Terraform version setting", name)
			}
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found terraform version: %s", terraformVersion))
		}
	}

	if terraformVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing terraform version", filename)
	}

	if tofuVersion == "" {
		return nil, fmt.Errorf("Cannot unmarshal infrastructure versions file %s: missing opentofu version", filename)
	}

	res := make(settingsStore)

	noneProviderKeys := map[string]struct{}{
		opentofuKey:  {},
		terraformKey: {},
	}

	for name, rawSettings := range infrastructureProviders {
		if _, ok := noneProviderKeys[name]; ok {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found non-provider-name key %s", name))
			continue
		}

		set, err := simpleFromMap(rawSettings, terraformVersion, tofuVersion)
		if err != nil {
			return nil, fmt.Errorf("Cannot unmarshal infrastructure settings for provider %s: %v", name, err)
		}

		cloudName := strings.ToLower(set.CloudName())

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found provider settings for %s: %s", name, cloudName))

		res[cloudName] = set
	}

	return res, nil
}
